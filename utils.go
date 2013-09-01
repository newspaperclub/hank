package main

import (
	"os"
	"log"
	"launchpad.net/goamz/s3"
	"path/filepath"
)

func checkFilePresence(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}

		log.Fatal(err)
	}

	file.Close()
	return true
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}

	return info.Size()
}

func pathForKey(rootPath string, key *s3.Key) string {
	return filepath.Join(rootPath, key.Key)
}
