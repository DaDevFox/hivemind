package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// map of parent directories to sync; TODO: dedup over time
var WorkCache map[string][]string
var TransferQueue chan struct {
	dest string
	src  string
}
var OutpropQueue chan string

func scan() {
	if WorkCache == nil {
		WorkCache = make(map[string][]string)
	}

	err := filepath.Walk(RootDir, func(path string, fi os.FileInfo, err error) error {
		f, err := os.Stat(path)
		if err != nil || f.IsDir() {
			return err
		}
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
			outprop, err := SubElem(core_dir, path)
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

					fmt.Printf("%s updated!", filename)
					// representational path stored, NOT actual
					OutpropQueue <- filepath.Join(filepath.Dir(path), *transformed_filename)
					// FLAG: on new directory add in core space; check for external matches??
				} else {
					untransformed_filename := transaction_type.satelliteToCore(filename)

					if untransformed_filename == nil {
						continue
					}

					// fmt.Println(*untransformed_filename)

					if !changed(path) {
						continue
					}

					// fmt.Printf("%s updated!", filename)

					// representational path stored, NOT actual
					TransferQueue <- struct {
						dest string
						src  string
					}{
						dest: filepath.Join(core_dir, *untransformed_filename),
						src:  path,
					}

					// DONE: from hashdb, find core path location of file with untransformed_filename
					// TODO: figure out how to resolve nonunique file names
					hashtable_lock.RLock()
					core_filepaths, ok := HASHDB_file_table[*untransformed_filename]
					hashtable_lock.Unlock()

					if !ok {
						continue
					}
					core_filepath_parent := filepath.Base(core_filepaths[0])

					fmt.Println("Adding to workcache")

					_, ok = WorkCache[core_dir]
					if !ok {
						WorkCache[core_filepath_parent] = make([]string, 0)
					}

					WorkCache[core_filepath_parent] = append(WorkCache[core_filepath_parent], filepath.Dir(path))
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
