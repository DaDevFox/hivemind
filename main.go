package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

const core_config_default_filename = "core.hive"
const satellite_config_default_filename = ".hive"

var root_dir string
var core_config string

var watcher *fsnotify.Watcher

func init_log() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.WarnLevel)
}

func copy() {
	buf := make([]byte, BUFFERSIZE)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}
}

// main
func main() {
	if len(os.Args) < 1 {
		fmt.Printf("Provide root_dir [config file]")
		return
	}

	root_dir := os.Args[0]
	if len(os.Args) > 1 {
		core_config := os.Args[1]
	} else {
		core_config := path.Join(root_dir, core_config_default_filename)
	}

	core_config_stream, err := os.Open(core_config)
	if err != nil {
		log.Fatal(err)
	}

	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	// starting at the root of the project, walk each file/directory searching for
	// directories
	if err := filepath.Walk("/Users/skdomino/Desktop/test", watchDir); err != nil {
		fmt.Println("ERROR", err)
	}

	done := make(chan bool)

	go func() {
		for {
			select {
			// watch for events
			//
			case event := <-watcher.Events:
				fmt.Printf("EVENT! %#v\n", event)

				// watch for errors
			case err := <-watcher.Errors:
				fmt.Println("ERROR", err)
			}
		}
	}()

	<-done
}

// watchDir gets run as a walk func, searching for directories to add watchers to
func watchDir(path string, fi os.FileInfo, err error) error {

	// since fsnotify can watch all the files in a directory, watchers only need
	// to be added to each nested directory
	if fi.Mode().IsDir() {
		return watcher.Add(path)
	}

	return nil
}
