package main

import (
	"io"
	"launchpad.net/goamz/s3"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const fileConcurrency = 1

type bucketDownloader struct {
	seenFilePaths     map[string]bool
	syncedFiles       uint64
	syncedBytes       uint64
	totalFiles        uint64
	totalBytes        uint64
	bucket            *s3.Bucket
	concurrency       uint
	targetPath        string
	keyQueue          chan *s3.Key
	keyWaitGroup      sync.WaitGroup
	downloadQueue     chan *s3.Key
	downloadWaitGroup sync.WaitGroup
}

func newBucketDownloader(bucket *s3.Bucket, targetPath string, concurrency uint) *bucketDownloader {
	downloader := bucketDownloader{
		bucket:        bucket,
		targetPath:    targetPath,
		concurrency:   concurrency,
		seenFilePaths: make(map[string]bool),
	}

	return &downloader
}

func (downloader *bucketDownloader) run() map[string]bool {
	downloader.startKeyWorkers()
	downloader.startDownloadWorkers()
	downloader.listContents()
	downloader.stopKeyWorkers()
	downloader.stopDownloadWorkers()

	return downloader.seenFilePaths
}

func (downloader *bucketDownloader) listContents() {
	marker := ""

	for {
		sourceList, err := downloader.bucket.List("", "", marker, 1000)
		if err != nil {
			log.Fatal(err)
		}

		for i := 0; i < len(sourceList.Contents); i++ {
			key := sourceList.Contents[i]
			downloader.keyQueue <- &key
			downloader.seenFilePaths[key.Key] = true
		}

		if !sourceList.IsTruncated {
			break
		}

		lastIndex := len(sourceList.Contents) - 1
		lastKey := sourceList.Contents[lastIndex]
		marker = lastKey.Key
	}
}

// Start up n goroutines to check whether s3.Keys on the fileQueue exist and
// match size
func (downloader *bucketDownloader) startKeyWorkers() {
	downloader.keyQueue = make(chan *s3.Key, 10000)

	for i := 0; i < fileConcurrency; i++ {
		downloader.keyWaitGroup.Add(1)
		go downloader.handleKeyQueue()
	}
}

func (downloader *bucketDownloader) stopKeyWorkers() {
	for i := 0; i < fileConcurrency; i++ {
		downloader.keyQueue <- nil
	}

	downloader.keyWaitGroup.Wait()
}

func (downloader *bucketDownloader) startDownloadWorkers() {
	downloader.downloadQueue = make(chan *s3.Key, 10000)

	for i := 0; i < int(downloader.concurrency); i++ {
		downloader.downloadWaitGroup.Add(1)
		go downloader.handleDownloadQueue()
	}
}

func (downloader *bucketDownloader) stopDownloadWorkers() {
	for i := 0; i < int(downloader.concurrency); i++ {
		downloader.downloadQueue <- nil
	}

	downloader.downloadWaitGroup.Wait()
}

func (downloader *bucketDownloader) handleKeyQueue() {
	for {
		key := <-downloader.keyQueue
		if key == nil {
			downloader.keyWaitGroup.Done()
			break
		}

		downloader.checkKey(key)

		atomic.AddUint64(&downloader.totalFiles, 1)
		atomic.AddUint64(&downloader.totalBytes, uint64(key.Size))
	}
}

func (downloader *bucketDownloader) handleDownloadQueue() {
	for {
		key := <-downloader.downloadQueue
		if key == nil {
			downloader.downloadWaitGroup.Done()
			break
		}

		downloader.downloadKey(key)

		atomic.AddUint64(&downloader.syncedFiles, 1)
		atomic.AddUint64(&downloader.syncedBytes, uint64(key.Size))
	}
}

func (downloader *bucketDownloader) checkKey(key *s3.Key) {
	path := pathForKey(downloader.targetPath, key)

	if checkFilePresence(path) {
		size := fileSize(path)

		if size == key.Size {
			return
		}

		log.Printf("Mismatched size for %v (expecting %d bytes, got %d bytes)", key.Key, key.Size, size)
	}

	downloader.downloadQueue <- key
}

func (downloader *bucketDownloader) downloadKey(key *s3.Key) {
	path := pathForKey(downloader.targetPath, key)

	// Make the dir if it doesn't exist
	dirPath := filepath.Dir(path)
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		log.Fatal(err)
	}

	fileWriter, err := os.Create(path)
	defer fileWriter.Close()
	if err != nil {
		log.Fatal(err)
	}

	bucketReader, err := downloader.bucket.GetReader(key.Key)
	defer bucketReader.Close()
	if err != nil {
		log.Fatal(err)
	}

	bytes, err := io.Copy(fileWriter, bucketReader)
	if err != nil {
		log.Fatal(err)
	}

	atomic.AddUint64(&downloader.syncedBytes, uint64(bytes))
	atomic.AddUint64(&downloader.syncedFiles, 1)

	log.Printf("Fetched %v (%d bytes)", key.Key, bytes)
}
