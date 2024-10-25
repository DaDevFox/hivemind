package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	// "hash"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-set/v3"
)

var HashTable_lock = sync.RWMutex{}
var FileTable_lock = sync.RWMutex{}

// path to hash
var HASHDB_hash_table map[string]string

// filename to path(s)
var HASHDB_file_table map[string](*set.TreeSet[string])

func hashdb_init() {
	// h = md5.New()
	HASHDB_hash_table = make(map[string]string)
	HASHDB_file_table = make(map[string](*set.TreeSet[string]))
}

func hashdb_diff(path string, update bool) bool {
	HashTable_lock.Lock()
	stored, exists := HASHDB_hash_table[path]
	newhash, err := md5sum(path)
	if err != nil {
		fmt.Printf("ERR while hashing: %s\n", err)
	}

	if !exists {
		fmt.Printf("detected new file: %s\n", path)
		if update {
			HASHDB_hash_table[path] = newhash
			hashdb_add_to_filetable(path)
		}
		HashTable_lock.Unlock()
		return true
	}
	HashTable_lock.Unlock()

	if update {

		HashTable_lock.Lock()
		HASHDB_hash_table[path] = newhash
		hashdb_add_to_filetable(path)
		HashTable_lock.Unlock()
	}

	return newhash != stored
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
	hash, err := md5sum(path)
	if err != nil {
		return err
	}
	HASHDB_hash_table[path] = hash
	hashdb_add_to_filetable(path)
	return nil
}

func md5sum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
