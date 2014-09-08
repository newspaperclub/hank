package main

import (
	"log"
	"os"
	"path/filepath"
)

type bucketDeleter struct {
	keepFilePaths map[string]bool
	deletedFiles  uint64
	deletedBytes  uint64
	targetPath    string
}

func NewBucketDeleter(targetPath string, keepFilePaths map[string]bool) *bucketDeleter {
	deleter := bucketDeleter{
		targetPath:    targetPath,
		keepFilePaths: keepFilePaths,
	}

	return &deleter
}

func (deleter *bucketDeleter) Run() {
	deleter.deleteFiles()
}

// Deletes files, if not present on the backup.
// TODO: cleanup empty directories.
func (deleter *bucketDeleter) deleteFiles() {
	visitFile := func(path string, fileInfo os.FileInfo, inputErr error) (err error) {
		// Skip over the file if it's not a normal, regular file
		if !fileInfo.Mode().IsRegular() {
			return nil
		}

		relativePath, err := filepath.Rel(deleter.targetPath, path)
		if err != nil {
			log.Fatal(err)
		}

		// Check if the file is to be kept
		if !deleter.keepFilePaths[relativePath] {
			err := os.Remove(path)
			if err != nil {
				log.Fatal(err)
			}

			deleter.deletedFiles += 1
			deleter.deletedBytes += uint64(fileInfo.Size())
		}

		return nil
	}

	err := filepath.Walk(deleter.targetPath, visitFile)
	if err != nil {
		log.Fatal(err)
	}
}
