package main

import (
	"log"
	"sync"
	"os"
	"io"
	"flag"
	"path/filepath"
	"launchpad.net/goamz/s3"
	"launchpad.net/goamz/aws"
)

var (
	waitGroup sync.WaitGroup
	fileQueue chan *string
	syncedFiles uint64
	syncedBytes uint64
	totalFiles uint64

	awsAccessKey string
	awsSecretKey string
	awsBucket string
	awsRegion string
	localRootPath string
)

func listBucket() {
	bucket := Bucket()
	marker := ""
	moreToFetch := true

	for moreToFetch {
		sourceList, err := bucket.List("", "", marker, 1000)
		if err != nil {
			log.Fatal(err)
		}

		//log.Printf("Received new list, found %v files", len(sourceList.Contents))

		for i := 0; i < len(sourceList.Contents); i++ {
			key := sourceList.Contents[i]
			fileQueue <- &key.Key
		}

		if !sourceList.IsTruncated {
			moreToFetch = false
		}

		lastIndex := len(sourceList.Contents) - 1
		lastKey := sourceList.Contents[lastIndex]
		marker = lastKey.Key
	}
}

func FileWorker(id int, fileQueue chan *string) {
	bucket := Bucket()

	for {
		key := <-fileQueue
		if key == nil {
			waitGroup.Done()
			break
		}

		localFilePath := filepath.Join(localRootPath, *key)

		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {

			// Make the dir if it doesn't exist
			dirPath := filepath.Dir(localFilePath)
			err := os.MkdirAll(dirPath, 0777)
			if err != nil {
				log.Fatal(err)
			}

			fileWriter, err := os.Create(localFilePath)
			defer fileWriter.Close()
			if err != nil {
				log.Fatal(err)
			}

			bucketReader, err := bucket.GetReader(*key)
			defer bucketReader.Close()
			if err != nil {
				log.Fatal(err)
			}

			bytes, err := io.Copy(fileWriter, bucketReader)
			if err != nil {
				log.Fatal(err)
			}

			syncedBytes += uint64(bytes)
			syncedFiles += 1

			log.Printf("Fetched %v (%d bytes)", *key, bytes)
		} else {
			log.Printf("Skipped %v", *key)
		}

		totalFiles += 1
	}
}

func Bucket() (bucket *s3.Bucket) {
	auth := aws.Auth{AccessKey: awsAccessKey, SecretKey: awsSecretKey}
	s3Conn := s3.New(auth, aws.Regions[awsRegion])
	return s3Conn.Bucket(awsBucket)
}

func initFlags() {
	flag.StringVar(&awsAccessKey, "access-key", "", "AWS Access Key")
	flag.StringVar(&awsSecretKey, "secret-key", "", "AWS Secret Key")
	flag.StringVar(&awsBucket, "bucket", "", "S3 Bucket")
	flag.StringVar(&awsRegion, "region", "", "S3 Region")

	flag.Parse()
	args := flag.Args()

	if len(args) > 0 {
		localRootPath = args[0]
	}

	if localRootPath == "" {
		log.Fatal("No destination provided.")
	}

	// If the provided path isn't absolute, make it so
	if !filepath.IsAbs(localRootPath) {
		workingDir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		localRootPath = filepath.Join(workingDir, localRootPath)
	}

	fileInfo, err := os.Stat(localRootPath)
	if os.IsNotExist(err) {
		log.Fatal("Destination not found.")
	}

	if !fileInfo.IsDir() {
		log.Fatal("Destination is not a directory.")
	}
}

func main() {
	initFlags()

	log.Printf("Starting bucket sync from %v to %v", awsBucket, localRootPath)

	fileQueue = make(chan *string, 2000)
	workerCount := 8

	// Fire up `workerCount` workers
	for i := 0; i < workerCount; i++ {
		waitGroup.Add(1)
		go FileWorker(i, fileQueue)
	}

	// Start the hard work
	listBucket()

	// Shutdown the workers by sending them a nil
	for i := 0; i < workerCount; i++ {
		fileQueue <- nil
	}

	// Wait for everything to finish up
	waitGroup.Wait()

	log.Printf("Synced %v/%v seen files, updated %v bytes", syncedFiles, totalFiles, syncedBytes)
}
