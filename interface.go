package main

import (
	// "encoding/json"

	// "github.com/davecgh/go-spew/spew"
	"path/filepath"

	"github.com/pterm/pterm"
)

var DEBUG_info bool = true
var area *pterm.AreaPrinter

func interface_init() {
	// Initialize a new PTerm area with fullscreen and center options
	// The Start() function returns the created area and an error (ignored here)
	a, _ := pterm.DefaultArea.WithRemoveWhenDone().Start()
	area = a
}

func interface_update() {
	res := ""
	res += pterm.Info.Sprintfln("Updating\n")

	for dir := range CONFIG_SourceDirs {
		panel := pterm.DefaultBox.WithTitleTopCenter(true).WithTitle(dir)

		body := ""

		if DEBUG_info {
			body += "---REGEX---\n"
			for _, transition := range CONFIG_SourceDirs[dir] {
				body += "hello -> " + *transition.coreToSatellite("hello") + "\n"
				body += (*transition.satelliteToCore(*transition.coreToSatellite("hello"))) + " <- " + *transition.coreToSatellite("hello") + "\n"
			}
			body += "-----------\n"
		}

		WorkCache_lock.RLock()

		for core_dir := range WorkCache {
			body += core_dir + "\n"
			for _, connection := range WorkCache[core_dir] {
				relpath, err := filepath.Rel(RootDir, connection)
				if err != nil {
					relpath = "ERROR"
				}
				body += "\t-->" + relpath + "\n"
			}
		}
		WorkCache_lock.RUnlock()

		res += panel.Sprint(body) + "\n"
	}

	if DEBUG_info {
		// res += pterm.DefaultBox.WithTitleTopCenter(true).WithTitle("file table").Sprint(spew.Sprint(HASHDB_file_table))
		// res += pterm.DefaultBox.WithTitleTopCenter(true).WithTitle("hashes").Sprint(json.Marshal(HASHDB_hash_table))
	}

	// area.Update(res)
}

func interface_cleanup() {
	area.Stop()
}
