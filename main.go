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

var RootDir string
var CoreConfig string

var watcher *fsnotify.Watcher

type coreToSatellite func(string) *string
type satelliteToCore func(string) *string

var CONFIG_SourceDirs map[string][]struct {
	coreToSatellite
	satelliteToCore
}

func init_log() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	f, err := os.OpenFile(
		"./output.log",
		os.O_CREATE,
		0644,
	)

	if err != nil {
		fmt.Printf("ERROR INITIALIZING LOGFILE: %s", err)
		panic(err)
	}
	log.SetOutput(f)

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
	init_log()

	if len(os.Args) < 1 {
		log.Printf("Provide root_dir [config file]\n")
		return
	}

	RootDir, _ = filepath.Abs(os.Args[1])
	if len(os.Args) > 2 {
		CoreConfig = os.Args[2]
	} else {
		CoreConfig = path.Join(RootDir, core_config_default_filename)
	}

	core_config_stream, err := os.Open(CoreConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer core_config_stream.Close()

	scanner := bufio.NewScanner(core_config_stream)
	scanner.Split(bufio.ScanLines)
	CONFIG_SourceDirs = make(map[string][]struct {
		coreToSatellite
		satelliteToCore
	})

	curr_dir := ""
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Trim(text, " \t") == "" {
			break
		}

		if curr_dir == "" || !(strings.HasPrefix(text, "\t") || strings.HasPrefix(text, "  ")) {
			curr_dir = text
			CONFIG_SourceDirs[curr_dir] = make([]struct {
				coreToSatellite
				satelliteToCore
			}, 0)
		} else {
			r_cts, _ := regexp.Compile(".+->(.+)")
			r_stc, _ := regexp.Compile("(.+)->.+")

			cts_text := strings.Trim(r_cts.FindStringSubmatch(text)[1], " \t")
			stc_text := strings.Trim(r_stc.FindStringSubmatch(text)[1], " \t")

			var cts coreToSatellite = func(s string) *string {
				var base string
				replaced_stc_text := strings.ReplaceAll(stc_text, "%s", "(.+)")
				r_matched_stc_text, _ := regexp.Compile(replaced_stc_text)
				matches := r_matched_stc_text.FindStringSubmatch(s)
				if len(matches) < 1 {
					return nil
				}

				// FLAG: stack var issues?
				base = matches[1]
				res := fmt.Sprintf(cts_text, base)
				return &res
			}

			var stc satelliteToCore = func(s string) *string {
				var base string
				replaced_cts_text := strings.ReplaceAll(cts_text, "%s", "(.+)")
				r_matched_cts_text, _ := regexp.Compile(replaced_cts_text)
				matches := r_matched_cts_text.FindStringSubmatch(s)
				if len(matches) < 1 {
					return nil
				}

				base = matches[1]

				// FLAG: stack var issues?
				res := fmt.Sprintf(stc_text, base)
				return &res
			}

			CONFIG_SourceDirs[curr_dir] = append(CONFIG_SourceDirs[curr_dir], struct {
				coreToSatellite
				satelliteToCore
			}{cts, stc})
		}
	}

	log.Printf("Hivemind spawning in %s; reading %s\n\n", RootDir, CoreConfig)
	watchdog_init()
	interface_init()
	defer interface_cleanup()

	// initial scan
	scan()

	// (one time update)
	interface_update()

	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	// starting at the root of the project, walk each file/directory searching for
	// directories
	if err := filepath.Walk(RootDir, watchDir); err != nil {
		fmt.Println("ERROR", err)
	}

	done := make(chan bool)

	go func() {
		for {
			select {
			// watch for events
			//
			case event := <-watcher.Events:
				// fmt.Printf("EVENT! %#v\n", event)
				go watchdog_process(event)
				interface_update()

				// watch for errors
			case err := <-watcher.Errors:
				// TODO: find way to prioritize over pterm
				log.Warn("ERROR", err)
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
