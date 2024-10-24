package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"
)

const core_config_default_filename = "core.hive"
const satellite_config_default_filename = ".hive"

var root_dir string
var core_config string

var watcher *fsnotify.Watcher

type coreToSatellite func(string) string
type satelliteToCore func(string) string

var CONFIG_source_dirs map[string][](chan struct {
	coreToSatellite
	satelliteToCore
})

func init_log() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.WarnLevel)
}

func copy(source os.File, destination os.File) error {
	BUFFERSIZE := 256
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

	return nil
}

// main
func main() {
	if len(os.Args) < 1 {
		fmt.Printf("Provide root_dir [config file]")
		return
	}

	root_dir := os.Args[1]
	if len(os.Args) > 2 {
		core_config = os.Args[2]
	} else {
		core_config = path.Join(root_dir, core_config_default_filename)
	}

	core_config_stream, err := os.Open(core_config)
	if err != nil {
		log.Fatal(err)
	}
	defer core_config_stream.Close()

	scanner := bufio.NewScanner(core_config_stream)
	scanner.Split(bufio.ScanLines)
	CONFIG_source_dirs := make(map[string](chan struct {
		coreToSatellite
		satelliteToCore
	}))

	curr_dir := ""
	for scanner.Scan() {
		text := scanner.Text()
		if curr_dir == "" || !strings.HasPrefix(text, "\t") {
			curr_dir = text
			CONFIG_source_dirs[curr_dir] = make((chan struct {
				coreToSatellite
				satelliteToCore
			}), 0)
		} else {
			r_cts, _ := regexp.Compile(".+->(.+)")
			r_stc, _ := regexp.Compile("(.+)->.+")

			cts_text := r_cts.FindStringSubmatch(text)[0]
			stc_text := r_stc.FindStringSubmatch(text)[0]

			var cts coreToSatellite = func(s string) string {
				return strings.ReplaceAll(cts_text, "%s", s)
			}

			var stc satelliteToCore = func(s string) string {
				return strings.ReplaceAll(stc_text, "%s", s)
			}

			(CONFIG_source_dirs[curr_dir]) <- struct {
				coreToSatellite
				satelliteToCore
			}{cts, stc}
		}

	}

	log.Printf("Hivemind spawning in %s; reading %s\n\n", root_dir, core_config)
	interface_init()
	defer interface_cleanup()

	// (one time update)
	interface_update()

	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	// starting at the root of the project, walk each file/directory searching for
	// directories
	if err := filepath.Walk(root_dir, watchDir); err != nil {
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
