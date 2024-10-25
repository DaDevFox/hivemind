package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-set/v3"
)

var HashTable_lock = sync.RWMutex{}

// path to hash
var HASHDB_hash_table map[string][]byte

// filename to path(s)
var HASHDB_file_table map[string](*set.TreeSet[string])

var h hash.Hash

func hashdb_init() {
	h = md5.New()
	HASHDB_hash_table = make(map[string][]byte)

	HASHDB_file_table = make(map[string](*set.TreeSet[string]))
}

func hashdb_diff(path string, update bool) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}

	_, err = io.Copy(h, file)
	if err != nil {
		return false
	}

	h.Reset()
	newhash := h.Sum(nil)

	HashTable_lock.Lock()
	_, exists := HASHDB_hash_table[path]
	if !exists {
		fmt.Printf("detected new file: %s\n", path)
		if update {
			HASHDB_hash_table[path] = newhash
			hashdb_add_to_filetable(path)
		}
		HashTable_lock.Unlock()
		return true
	}

	stored := HASHDB_hash_table[path]
	HashTable_lock.Unlock()

	if update {
		h.Reset()

		HashTable_lock.Lock()
		HASHDB_hash_table[path] = newhash
		hashdb_add_to_filetable(path)
		HashTable_lock.Unlock()
	}

	return !bytes.Equal(newhash, stored)
}

func hashdb_add_to_filetable(path string) {
	filename := filepath.Base(path)
	_, ok := HASHDB_file_table[filename]
	if !ok {
		HASHDB_file_table[filename] = set.NewTreeSet(func(s1, s2 string) int {
			for core_dir := range CONFIG_SourceDirs {
				subelem, _ := SubElem(core_dir, s1)
				if subelem {
					return 1
				}
				subelem, _ = SubElem(core_dir, s2)
				if subelem {
					return -1
				}
			}

			return 0
		})
	}
	HASHDB_file_table[filename].Insert(path)

}

func hashdb_update(path string) error {
	h.Reset()

	file, err := os.Open(path)
	if err != nil {
		return err
	}

	_, err = io.Copy(h, file)
	if err != nil {
		return err
	}

	HASHDB_hash_table[path] = h.Sum(nil)
	hashdb_add_to_filetable(path)
	return nil
}
