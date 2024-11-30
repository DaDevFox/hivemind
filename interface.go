package main

import (
	// "encoding/json"

	// "github.com/davecgh/go-spew/spew"
	// "io"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	// "github.com/pterm/pterm/putils"
)

var logger *pterm.Logger

var DEBUG_info bool = true
var area *pterm.AreaPrinter

var CHECKING_source_dirs []string
var WORKING_source_dirs []string

func interface_init() {
	// Initialize a new PTerm area with fullscreen and center options
	// The Start() function returns the created area and an error (ignored here)
	area, _ = pterm.DefaultArea.WithCenter().Start()
	logger = &pterm.DefaultLogger

	logo, _ := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("Hive", pterm.NewStyle(pterm.FgYellow)),
		putils.LettersFromStringWithStyle("Mind", pterm.NewStyle(pterm.FgBlack))).
		Srender()

	pterm.DefaultCenter.Print(logo)
}

var count = 0

func interface_update() {
	res := ""
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

	area.Update(res)
	count += 1
}

func interface_cleanup() {
	area.Stop()
}
