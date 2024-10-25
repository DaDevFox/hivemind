package main

import (
	"fmt"
	"io"
	"os"
	"sync"
)

func copyFileThreadSafe(src, dst string, mutex *sync.Mutex) error {
	mutex.Lock()
	defer mutex.Unlock()

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	n, err := io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	fmt.Printf("%d bytes copied from %s to %s\n", n, src, dst)
	return nil
}
