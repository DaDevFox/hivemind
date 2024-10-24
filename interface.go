package main

import (
	"time"

	"github.com/pterm/pterm"
)

var area *pterm.AreaPrinter

func interface_init() {
	// Initialize a new PTerm area with fullscreen and center options
	// The Start() function returns the created area and an error (ignored here)
	a, _ := pterm.DefaultArea.WithRemoveWhenDone().Start()
	area = a

}

func interface_update() {
	res := ""

	for dir := range CONFIG_SourceDirs {
		panel := pterm.DefaultBox.WithTitleTopCenter(true).WithTitle(dir)
		res += panel.Sprint()
		res += pterm.Info.Sprintln("test")

		// Pause for a second
		time.Sleep(time.Second)
	}

	res += pterm.Info.Sprintfln("Updating")

	area.Update(res)
}

func interface_cleanup() {
	area.Stop()
}
