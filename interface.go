package main

import (
	"time"

	"github.com/pterm/pterm"
)

var area *pterm.AreaPrinter

func interface_init() {
	// Initialize a new PTerm area with fullscreen and center options
	// The Start() function returns the created area and an error (ignored here)
	a, _ := pterm.DefaultArea.WithFullscreen().WithCenter().Start()
	area = a

}

func interface_update() {

	// Loop 5 times to demonstrate dynamic content update
	// Update the content of the area with the current count
	for i := 0; i < 5; i++ {
		// The Sprintf function is used to format the string with the count
		area.Update(pterm.Sprintf("Current count: %d\nAreas can update their content dynamically!", i))

		// Pause for a second
		time.Sleep(time.Second)
	}

}

func interface_cleanup() {
	area.Stop()
}
