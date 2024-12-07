package main

import (
	// "encoding/json"

	// "github.com/davecgh/go-spew/spew"
	// "io"
	// "encoding/json"
	"os"
	"path/filepath"

	// "github.com/dgraph-io/badger/v4"
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
	// regex table
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

	// hash status
	hashedMatchingStyle := pterm.NewStyle(pterm.BgGreen, pterm.FgWhite)
	hashedWorkingStyle := pterm.NewStyle(pterm.BgYellow, pterm.FgWhite)
	hashedCheckingStyle := pterm.NewStyle(pterm.BgWhite, pterm.FgBlack)

	subitems, err := os.ReadDir(RootDir)
	if err != nil {
		log.Fatal("couldn't read subdirs of RootDir")
	}

	tree := pterm.TreeNode{
		Text: RootDir,
	}

	queue := make(chan *pterm.TreeNode, 2*BUFFER_SIZE)
	for _, v := range subitems {
		newNode := pterm.TreeNode{
			Text: v.Name(),
		}
		queue <- &newNode
		tree.Children = append(tree.Children, newNode)
	}

	for len(queue) > 0 {
		item := <-queue
		working := false
		checking := false
		for _, checkingWorkItem := range WORKING_source_dirs {
			subelem, err := SubElem(item.Text, checkingWorkItem)
			if err != nil {
				log.Fatal("rendering error")
			}
			working = working || subelem
		}
		for _, checkingCheckingItem := range WORKING_source_dirs {
			subelem, err := SubElem(item.Text, checkingCheckingItem)
			if err != nil {
				log.Fatal("rendering error")
			}
			checking = checking || subelem
		}

		if working {
			item.Text = hashedWorkingStyle.Sprint(item.Text)
		} else if checking {
			item.Text = hashedCheckingStyle.Sprint(item.Text)
		} else {
			item.Text = hashedMatchingStyle.Sprint(item.Text)
		}
	}

	stree, err := pterm.DefaultTree.WithRoot(tree).Srender()
	if err != nil {
		log.Fatal("rendering error")
	}

	// pterm.DefaultTree.WithRoot(tree).Render()
	// pterm.Info.Print("TEST")
	// pterm.Info.Print(stree)

	res += stree

	// err = HASHDB_hash_table.View(func(txn *badger.Txn) error {
	// 	opts := badger.DefaultIteratorOptions
	// 	opts.PrefetchSize = 10
	// 	it := txn.NewIterator(opts)
	// 	defer it.Close()
	// 	for it.Rewind(); it.Valid(); it.Next() {
	// 		item := it.Item()
	// 		k := item.Key()
	// 		err := item.Value(func(v []byte) error {
	// 			hashedMatchingStyle.Printfln("%s", string(k[:]))
	// 			return nil
	// 		})
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// 	return nil
	// })
	//
	// if err != nil {
	// 	log.Fatal("error")
	// }

	area.Update(res)
	count += 1
}

func interface_cleanup() {
	area.Stop()
}
