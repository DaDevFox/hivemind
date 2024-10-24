package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

var BUFFER_SIZE = 2048

// map of parent directories to sync; TODO: dedup over time
var WorkCache_lock = sync.RWMutex{}

var WorkCache map[string][]string
var TransferQueue chan struct {
	dest string
	src  string
}
var OutpropQueue chan string

func watchdog_init() {
	TransferQueue = make(chan struct {
		dest string
		src  string
	}, BUFFER_SIZE)
	OutpropQueue = make(chan string, BUFFER_SIZE)
}

func scan() {
	if WorkCache == nil {
		WorkCache_lock.Lock()
		WorkCache = make(map[string][]string)
		WorkCache_lock.Unlock()
	}

	err := filepath.Walk(RootDir, func(path string, fi os.FileInfo, err error) error {
		f, err := os.Stat(path)
		if err != nil || f.IsDir() {
			return err
		}

		// TODO: use ignore filter here
		return check(path)
	})
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

	go func() {
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

					if !changed(path) {
						continue
					}

					// representational path stored, NOT actual
					OutpropQueue <- filepath.Join(filepath.Dir(path), *transformed_filename)
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
					hashtable_lock.RLock()
					core_filepaths, ok := HASHDB_file_table[*untransformed_filename]

					if !ok {
						fmt.Println("error file changed and mapped to core dir file; file does not exist in core dir")
						continue
					}

					// core_filepath_parent := ""
					// for path := range core_filepaths.Items {
					// }

					core_filepath_parent := filepath.Dir(core_filepaths.TopK(1)[0])
					hashtable_lock.RUnlock()

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
	}()
	return nil
}

func on_change(event fsnotify.Event) {
	// check if file or dir
	check(event.Name)
}

func transfer(event struct {
	dest string
	src  string
}) {

}

func transfer_serve() {
	for s := range TransferQueue {
		// code (compares hashes from hashdb.go)
		go transfer(s)
	}
}
