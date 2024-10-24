package main

import (
	"bytes"
	"crypto/md5"
	"hash"
	"io"
	"os"
	"path/filepath"
)

// path to hash
var HASHDB_hash_table map[string][]byte

// filename to path(s)
var HASHDB_file_table map[string][]string

var h hash.Hash

func hashdb_init() {
	h = md5.New()
	HASHDB_hash_table = make(map[string][]byte)
	HASHDB_file_table = map[string][]string{}
}

func hashdb_diff(path string, update bool) bool {
	h.Reset()

	file, err := os.Open(path)
	if err != nil {
		return false
	}

	_, err = io.Copy(h, file)
	if err != nil {
		return false
	}

	_, exists := HASHDB_hash_table[path]

	newhash := h.Sum(nil)
	stored := make([]byte, 0)
	if exists {
		stored = HASHDB_hash_table[path]
	}

	if update {
		HASHDB_hash_table[path] = h.Sum(nil)
		filename := filepath.Base(path)
		_, ok := HASHDB_file_table[filename]
		if !ok {
			HASHDB_file_table[filename] = make([]string, 0)
		}
		HASHDB_file_table[filename] = append(HASHDB_file_table[filename], path)
	}

	if exists {
		return !bytes.Equal(newhash, stored)
	} else {
		return true
	}
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
	filename := filepath.Base(path)
	_, ok := HASHDB_file_table[filename]
	if !ok {
		HASHDB_file_table[filename] = make([]string, 0)
	}
	HASHDB_file_table[filename] = append(HASHDB_file_table[filename], path)

	return nil
}
