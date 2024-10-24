package main

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// map of parent directories to sync; TODO: dedup over time
var WorkCache map[string]string
var TransferQueue chan struct {
	dest string
	src  string
}

func scan() {
	if WorkCache == nil {
		WorkCache = make(map[string]string)
	}

	err := filepath.Walk(RootDir, check)
	if err != nil {
		logrus.Fatal(err)
	}
}

func check(path string, fi os.FileInfo, err error) error {
	go func() {
		// code (compares hashes from hashdb.go)
	}()
	return nil
}

func on_change(event fsnotify.Event) {

}

func transfer(event struct {
	dest string
	src  string
}) {

}

func transfer_serve() {
	for s := range TransferQueue {
		go transfer(s)
	}
}
