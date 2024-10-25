package main

import (
	"fmt"
	// "io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

var file_copy_mutex = sync.Mutex{}

var BUFFER_SIZE = 2048

// map of parent directories to sync; TODO: dedup over time
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

func watchdog_init() {
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
}

func scan() {
	if WorkCache == nil {
		WorkCache_lock.Lock()
		WorkCache = make(map[string][]string)
		WorkCache_lock.Unlock()
	}

	// perform full transfer/balancing check
	err := filepath.Walk(RootDir, func(path string, fi os.FileInfo, err error) error {
		f, err := os.Stat(path)
		if err != nil || f.IsDir() {
			return err
		}

		// TODO: use ignore filter here
		go check(path)

		return err
	})

	// perform full write/transfer operations

	fmt.Println("completed full directory sync")

	if err != nil {
		logrus.Fatal(err)
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
	// DONE: hook up to hash db; return file hash changed or is different from other
	changed := func(file string) bool {
		return hashdb_diff(file, true)
	}

	filename := filepath.Base(path)
	for core_dir, matches := range CONFIG_SourceDirs {
		abs_path, err := filepath.Abs(path)
		abs_core_dir, err := filepath.Abs(core_dir)
		if err != nil {
			logrus.Fatal(err)
		}
		outprop, err := SubElem(abs_core_dir, abs_path)
		if err != nil {
			logrus.Fatal(err)
		}
		for _, transaction_type := range matches {
			if outprop {
				transformed_filename := transaction_type.coreToSatellite(filename)
				if transformed_filename == nil {
					continue
				}

				// TODO: or below with diff relative to ANY satellite mirror
				if !changed(path) {
					continue
				}

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

				fmt.Printf("located %s mapping to %s; checking\n", path, *untransformed_filename)
				if !changed(path) {
					continue
				}

				fmt.Printf("change detected: %s\n", *untransformed_filename)

				// fmt.Printf("%s updated!", filename)

				// DONE: from hashdb, find core path location of file with untransformed_filename
				// TODO: figure out how to resolve nonunique file names
				FileTable_lock.Lock()
				core_filepaths, ok := HASHDB_file_table[*untransformed_filename]

				if !ok {
					fmt.Println("error file changed and mapped to core dir file; file does not exist in core dir")
					continue
				}

				// core_filepath_parent := ""
				// for path := range core_filepaths.Items {
				// }

				core_filepath_parent := filepath.Dir(core_filepaths.TopK(1)[0])
				FileTable_lock.Unlock()

				TransferQueue <- struct {
					dest string
					src  string
				}{
					dest: filepath.Join(core_filepath_parent, *untransformed_filename),
					src:  path,
				}

				WorkCache_lock.Lock()
				fmt.Printf("Adding %s/* <-> %s/* to workcache\n", filepath.Dir(path), core_filepath_parent)
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

	interface_update()
	return nil
}

func watchdog_process(event fsnotify.Event) {
	go check(event.Name)
}

func transfer(event struct {
	dest string
	src  string
}) error {
	// src_file, err := os.Open(event.src)
	// defer src_file.Close()
	// if err != nil {
	// 	return err
	// }

	// TODO: recovery flag + backup for contents of file prior to overwrite
	OutpropQueue <- struct {
		dest_filename string
		src           string
	}{
		dest_filename: filepath.Base(event.src), // TODO: expand across match patterns for outprop requests
		src:           event.dest,
	}

	// dest_file, err := os.Create(event.dest)
	// defer dest_file.Close()
	// if err != nil {
	// 	logrus.Fatal(err)
	// }
	//
	err := copyFileThreadSafe(event.src, event.dest, &file_copy_mutex)
	if err != nil {
		fmt.Printf("ERR: %s", err)
	}

	// n, err := io.Copy(dest_file, src_file)
	// fmt.Printf("%d bytes copied from %s to %s\n", n, event.src, event.dest)

	return nil
}

func transfer_serve() {
	for s := range TransferQueue {
		fmt.Printf("serving %s\n", s.dest)
		go transfer(s)
	}
}

func outpropogate(event struct {
	dest_filename string
	src           string
}) error {
	// src, err := os.Open(event.src)
	parent := filepath.Dir(event.src)
	// defer src.Close()
	// if err != nil {
	// 	fmt.Print(err)
	// }
	// fmt.Println("TEST")
	//

	WorkCache_lock.RLock()
	for _, mapping := range WorkCache[parent] {
		other_path := filepath.Join(mapping, event.dest_filename)
		fmt.Println(other_path)

		// dest, err := os.Create(other_path)
		// defer dest.Close()
		// if err != nil {
		// 	return err
		// }
		//
		// n, err := io.Copy(dest, src)

		err := copyFileThreadSafe(event.src, other_path, &file_copy_mutex)
		if err != nil {
			fmt.Printf("ERR: %s", err)
		}

		// fmt.Printf("%d bytes copied from %s to %s\n", n, event.src, other_path)
	}

	WorkCache_lock.RUnlock()

	return nil
}

func outprop_serve() {
	for e := range OutpropQueue {
		go outpropogate(e)
	}
}
