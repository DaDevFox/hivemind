package main

import (
	"crypto/md5"
	"encoding/hex"
	"path"

	// "hash"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/hashicorp/go-set/v3"
)

// var HashTable_lock = sync.RWMutex{}
var FileTable_lock = sync.RWMutex{}

// stores path to hash (unique) relation
var HASHDB_hash_table *badger.DB

// stores filename to path(s) (nonunique/not O(1)) relation
var HASHDB_file_table map[string](*set.TreeSet[string])

func hashdb_init() {
	// h = md5.New()
	hashdb_load()
	// HASHDB_hash_table = make(map[string]string)
	HASHDB_file_table = make(map[string](*set.TreeSet[string]))
}

// COPIES and returns value from hashdb hash table persistent store
func hashdb_table_get(path string) string {
	var value []byte = nil
	HASHDB_hash_table.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(path))
		if err != nil {
			return err
		}
		item.Value(func(val []byte) error {
			// This func with val would only be called if item.Value encounters no error.

			// Copying or parsing val is valid.
			value = make([]byte, len(val))
			value = append(value, val...)

			return nil
		})

		return nil
	})
	return string(value[:])
}

func hashdb_table_set(path string, value string) error {
	return HASHDB_hash_table.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(path), []byte(value))
		err := txn.SetEntry(e)
		return err
	})
}

func hashdb_table_exists(path string) bool {
	exists := false
	HASHDB_hash_table.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(path))
		exists = err != badger.ErrKeyNotFound

		return nil
	})
	return exists
}

func hashdb_load() {
	// load hash table from file
	dbPath := path.Join(RootDir, ".hive")

	log.Info(dbPath)
	var err error = nil
	HASHDB_hash_table, err = badger.Open(badger.DefaultOptions(dbPath).
		// prepare a default of 10MB pre-allocated storage
		WithValueLogFileSize(10 << 20).
		// store 16-bytes (128 bits) for md5 hashes as values
		WithValueThreshold(16).
		// and compress
		WithCompression(options.ZSTD).
		WithZSTDCompressionLevel(3))
	if err != nil {
		log.Fatal("unable to open badger database")
	}

	// load hash table entries into file table

}

func hashdb_diff(path string, update bool) bool {
	// HashTable_lock.Lock()
	exists := hashdb_table_exists(path)

	stored := hashdb_table_get(path)
	newhash, err := md5sum(path)
	if err != nil {
		log.Printf("ERR while hashing: %s\n", err)
		// test
		// return false
	}

	if !exists {
		log.Printf("detected new file: %s\n", path)
		if update {
			hashdb_table_set(path, newhash)
			hashdb_add_to_filetable(path)
		}
		// HashTable_lock.Unlock()
		return true
	}
	// HashTable_lock.Unlock()

	if update {

		// HashTable_lock.Lock()
		hashdb_table_set(path, newhash)
		hashdb_add_to_filetable(path)
		// HashTable_lock.Unlock()
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
	hashdb_table_set(path, hash)
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

func hashdb_cleanup() {
	HASHDB_hash_table.Close()
}
