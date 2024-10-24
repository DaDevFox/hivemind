package main

import (
	// "encoding/json"

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

			body += "---WORK CACHE---\n"
			for connection := range WorkCache {
				body += connection + "\n"
			}
		}
		res += panel.Sprint(body) + "\n"
	}

	if DEBUG_info {
		// res += pterm.DefaultBox.WithTitleTopCenter(true).WithTitle("file table").Sprint(json.Marshal(HASHDB_file_table))
		// res += pterm.DefaultBox.WithTitleTopCenter(true).WithTitle("hashes").Sprint(json.Marshal(HASHDB_hash_table))
	}

	area.Update(res)
}

func interface_cleanup() {
	area.Stop()
}
