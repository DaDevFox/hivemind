package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var file_copy_mutex = sync.Mutex{}

var BUFFER_SIZE = 2048

// map of parent directories to sync; TODO: dedup over time
// TODO: save to/read from workcache file next to core.hive
var WorkCache_lock = sync.RWMutex{}

var WorkCache map[string][]string
var TransferQueue chan struct {
	dest string
	src  string
}
var OutpropQueue chan struct {
	dest_filename string
	src           string
}

func watchdog_cleanup() {
	hashdb_cleanup()
}

func watchdog_init() {
	hashdb_init()
	TransferQueue = make(chan struct {
		dest string
		src  string
	}, BUFFER_SIZE)
	OutpropQueue = make(chan struct {
		dest_filename string
		src           string
	}, BUFFER_SIZE)
	go transfer_serve()
	go outprop_serve()

	for core_dir := range CONFIG_SourceDirs {
		err := filepath.Walk(core_dir, func(path string, fi os.FileInfo, err error) error {
			for _, excludePattern := range CONFIG_IgnoreGlobs {
				relpath, err := filepath.Rel(RootDir, path)
				if err != nil {
					return err
				}

				if fi.IsDir() && excludePattern.Match(relpath) {
					return filepath.SkipDir // Skip this directory
				}
			}

			if fi.IsDir() {
				return nil
			}

			hashdb_update(path)

			return nil
		})

		if err != nil {
			log.Warn(err)
		}
	}
}

func scan() {
	log.Info("Performing scan")
	if WorkCache == nil {
		WorkCache_lock.Lock()
		WorkCache = make(map[string][]string)
		WorkCache_lock.Unlock()
	}

	// perform full transfer/balancing check
	err := filepath.Walk(RootDir, func(path string, fi os.FileInfo, err error) error {
		for _, excludePattern := range CONFIG_IgnoreGlobs {
			relpath, err := filepath.Rel(RootDir, path)
			if err != nil {
				return err
			}

			if fi.IsDir() && excludePattern.Match(relpath) {
				log.Printf("Ignoring %s", relpath)
				return filepath.SkipDir // Skip this directory
			}
		}

		if fi.IsDir() {
			return nil
		}

		log.Printf("%s", path)
		go check(path)

		return nil
	})

	// perform full write/transfer operations

	log.Println("completed full directory sync")

	if err != nil {
		log.Fatal(err)
	}
}

func SubElem(parent, sub string) (bool, error) {
	up := ".." + string(os.PathSeparator)

	// path-comparisons using filepath.Abs don't work reliably according to docs (no unique representation).
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, nil
	}
	return false, nil
}

func check(path string) error {
	changed := func(file string) bool {
		return hashdb_diff(file, true)
	}

	if !changed(path) {
		return nil
	}

	filename := filepath.Base(path)
	for core_dir, matches := range CONFIG_SourceDirs {
		abs_path, err := filepath.Abs(path)
		abs_core_dir, err := filepath.Abs(core_dir)
		if err != nil {
			log.Fatal(err)
		}
		outprop, err := SubElem(abs_core_dir, abs_path)
		if err != nil {
			log.Fatal(err)
		}
		for _, transaction_type := range matches {
			if outprop {
				transformed_filename := transaction_type.coreToSatellite(filename)
				if transformed_filename == nil {
					continue
				}

				// TODO: or below with diff relative to ANY satellite mirror
				// if !changed(path) {
				// 	continue
				// }

				// representational path stored, NOT actual
				OutpropQueue <- struct {
					dest_filename string
					src           string
				}{
					dest_filename: *transformed_filename,
					src:           path,
				}
				// FLAG: on new directory add in core space; check for external matches??
			} else {
				untransformed_filename := transaction_type.satelliteToCore(filename)

				if untransformed_filename == nil {
					continue
				}

				log.Printf("located %s mapping to %s; checking\n", path, *untransformed_filename)

				log.Printf("change detected: %s\n", *untransformed_filename)

				//log.Printf("%s updated!", filename)

				// DONE: from hashdb, find core path location of file with untransformed_filename
				// TODO: figure out how to resolve nonunique file names
				FileTable_lock.Lock()
				core_filepaths, ok := HASHDB_file_table[*untransformed_filename]

				if !ok {
					log.Println("error file changed and mapped to core dir file; file does not exist in core dir")
					continue
				}

				// core_filepath_parent := ""
				// for path := range core_filepaths.Items {
				// }

				core_filepath_parent := filepath.Dir(core_filepaths.TopK(1)[0])
				FileTable_lock.Unlock()

				log.Printf("adding to transferqueue: %s\n", *untransformed_filename)
				destPath := filepath.Join(core_filepath_parent, *untransformed_filename)
				TransferQueue <- struct {
					dest string
					src  string
				}{
					dest: destPath,
					src:  path,
				}

				WorkCache_lock.Lock()
				log.Printf("Adding %s/* <-> %s/* to workcache\n", filepath.Dir(path), core_filepath_parent)
				_, ok = WorkCache[core_filepath_parent]
				if !ok {
					WorkCache[core_filepath_parent] = make([]string, 0)
				}

				// TODO: convert to map[string]set[string]
				WorkCache[core_filepath_parent] = append(WorkCache[core_filepath_parent], filepath.Dir(path))
				WorkCache_lock.Unlock()
			}
		}

	}

	return nil
}

func watchdog_process(event fsnotify.Event) {
	go check(event.Name)
}

func transfer(event struct {
	dest string
	src  string
}) error {
	CHECKING_queue <- CheckupEvent{
		path:  event.src,
		added: true,
	}
	defer func() {
		CHECKING_queue <- CheckupEvent{
			path:  event.src,
			added: false,
		}
	}()

	log.Printf("Transferring %s\n", event.src)
	// TODO: recovery flag + backup for contents of file prior to overwrite
	err := copyFileThreadSafe(event.src, event.dest, &file_copy_mutex)
	if err != nil {
		log.Printf("ERR: %s", err)
	}

	OutpropQueue <- struct {
		dest_filename string
		src           string
	}{
		dest_filename: filepath.Base(event.src), // TODO: expand across match patterns for outprop requests
		src:           event.dest,
	}

	CHECKING_queue <- CheckupEvent{
		path:  event.src,
		added: true,
	}

	return nil
}

func transfer_serve() {
	for s := range TransferQueue {
		log.Printf("serving %s\n", s.dest)
		go transfer(s)
	}
}

func outpropogate(event struct {
	dest_filename string
	src           string
}) error {
	WORKING_queue <- CheckupEvent{
		path:  event.src,
		added: true,
	}
	defer func() {
		WORKING_queue <- CheckupEvent{
			path:  event.src,
			added: false,
		}
	}()

	log.Printf("Outpropogating %s\n", event.dest_filename)
	parent := filepath.Dir(event.src)
	WorkCache_lock.RLock()
	for _, mapping := range WorkCache[parent] {
		log.Printf("2; Outpropogating %s\n", event.dest_filename)
		other_path := filepath.Join(mapping, event.dest_filename)
		log.Println(other_path)

		err := copyFileThreadSafe(event.src, other_path, &file_copy_mutex)
		if err != nil {
			log.Printf("ERR: %s", err)
		}
	}

	WorkCache_lock.RUnlock()

	return nil
}

func outprop_serve() {
	for e := range OutpropQueue {
		go outpropogate(e)
	}
}
