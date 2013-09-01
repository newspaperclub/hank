package main

import (
	"flag"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"os"
	"path/filepath"
)

var (
	awsAccessKey        string
	awsSecretKey        string
	awsBucket           string
	awsRegion           = "eu-west-1"
	localRootPath       string
	downloadConcurrency uint = 8
)

func newBucketConnection() (bucket *s3.Bucket) {
	auth := aws.Auth{AccessKey: awsAccessKey, SecretKey: awsSecretKey}
	s3Conn := s3.New(auth, aws.Regions[awsRegion])
	return s3Conn.Bucket(awsBucket)
}

func initFlags() {
	flag.StringVar(&awsAccessKey, "access-key", "", "AWS Access Key")
	flag.StringVar(&awsSecretKey, "secret-key", "", "AWS Secret Key")
	flag.StringVar(&awsBucket, "bucket", "", "S3 Bucket")
	flag.StringVar(&awsRegion, "region", awsRegion, "S3 Region")

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

	bucket := newBucketConnection()
	downloader := NewBucketDownloader(bucket, localRootPath, downloadConcurrency)
	downloader.Run()

	log.Printf("Saw %d files (%d bytes), updated %d files (%d bytes)", downloader.totalFiles, downloader.totalBytes, downloader.syncedFiles, downloader.syncedBytes)
}
