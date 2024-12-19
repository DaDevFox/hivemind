package main

import (
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-set/v3"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

var logger *pterm.Logger

var DEBUG_info bool = true
var area *pterm.AreaPrinter

type CheckupEvent struct {
	path  string
	added bool
}

var updateMutex = sync.Mutex{}

var CHECKING_queue chan CheckupEvent
var WORKING_queue chan CheckupEvent
var CHECKING_source_dirs *set.TreeSet[string]
var WORKING_source_dirs *set.TreeSet[string]

func interface_init() {
	updateMutex.Lock()
	defer updateMutex.Unlock()
	CHECKING_queue = make(chan CheckupEvent, 2*BUFFER_SIZE)
	WORKING_queue = make(chan CheckupEvent, 2*BUFFER_SIZE)

	CHECKING_source_dirs = set.NewTreeSet(func(s1, s2 string) int {
		if s1 > s2 {
			return 1
		}
		return -1
	})
	WORKING_source_dirs = set.NewTreeSet(func(s1, s2 string) int {
		if s1 > s2 {
			return 1
		}
		return -1
	})

	// Initialize a new PTerm area with fullscreen and center options
	// The Start() function returns the created area and an error (ignored here)
	area, _ = pterm.DefaultArea.WithCenter().Start()
	logger = &pterm.DefaultLogger

	logo, _ := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("Hive", pterm.NewStyle(pterm.FgYellow)),
		putils.LettersFromStringWithStyle("Mind", pterm.NewStyle(pterm.FgBlack))).
		Srender()

	pterm.DefaultCenter.Print(logo)

	interface_serve()
}

var count = 0

func interface_serve() {
	go interface_checking_serve()
	go interface_working_serve()
}

func interface_checking_serve() {
	for event := range CHECKING_queue {
		updateMutex.Lock()
		if event.added {
			CHECKING_source_dirs.Insert(event.path)
		} else {
			CHECKING_source_dirs.Remove(event.path)
		}
		log.Infof("CHECKING (%t) %s", event.added, event.path)
		interface_update()
		updateMutex.Unlock()
	}
}

func interface_working_serve() {
	for event := range WORKING_queue {
		updateMutex.Lock()
		if event.added {
			WORKING_source_dirs.Insert(event.path)
		} else {
			WORKING_source_dirs.Remove(event.path)
		}
		log.Infof("WORKING (%t) %s", event.added, event.path)
		interface_update()
		updateMutex.Unlock()
	}
}

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

	tree := pterm.TreeNode{
		Text: RootDir,
	}

	queue := make(chan struct {
		*pterm.TreeNode
		int // child index (-1 for self)
		string
	}, 2*BUFFER_SIZE)
	queue <- struct {
		*pterm.TreeNode
		int
		string
	}{&tree, -1, RootDir}
	for len(queue) > 0 {
		item := <-queue

		itempath := item.string
		var node *pterm.TreeNode
		if item.int == -1 {
			node = item.TreeNode
		} else {
			node = &item.TreeNode.Children[item.int]
		}

		// check display mods necessary for item
		working := false
		checking := false
		for checkingWorkItem := range WORKING_source_dirs.Items() {
			subelem, err := SubElem(itempath, checkingWorkItem)
			if err != nil {
				log.Printf("rendering error: %s", err.Error())
			}
			working = working || subelem
		}
		for checkingCheckingItem := range WORKING_source_dirs.Items() {
			subelem, err := SubElem(itempath, checkingCheckingItem)
			if err != nil {
				log.Printf("rendering error: %s", err.Error())
			}
			checking = checking || subelem
		}

		// stat file -- don't render if unable to
		fi, err := os.Stat(itempath)
		if err != nil {
			log.Warnf("Couldn't stat path %s", itempath)
			continue
		}

		if fi.IsDir() && (working || checking || itempath == RootDir) {
			subitems, err := os.ReadDir(itempath)
			if err != nil {
				log.Fatalf("Error reading directory %s", itempath)
			}

			for _, v := range subitems {
				node.Children = append(node.Children, pterm.TreeNode{})
				childNode := &node.Children[len(node.Children)-1]
				childNode.Text = v.Name()
				childNode.Children = make([]pterm.TreeNode, 0)
				queue <- struct {
					*pterm.TreeNode
					int
					string
				}{node, len(node.Children) - 1, path.Join(itempath, v.Name())}
			}
		}

		if working {
			node.Text = hashedWorkingStyle.Sprintf("* %s", node.Text)
		} else if checking {
			node.Text = hashedCheckingStyle.Sprintf("+ %s", node.Text)
		} else {
			node.Text = hashedMatchingStyle.Sprint(node.Text)
		}
	}

	stree, err := pterm.DefaultTree.WithRoot(tree).Srender()
	if err != nil {
		log.Printf("rendering error: %s", err.Error())
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
