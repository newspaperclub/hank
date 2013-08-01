package main

import (
	"flag"
	"io"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var (
	fileQueue         chan *s3.Key
	fileWaitGroup     sync.WaitGroup
	downloadQueue     chan *s3.Key
	downloadWaitGroup sync.WaitGroup
	syncedFiles       uint64
	syncedBytes       uint64
	totalFiles        uint64
	totalBytes        uint64

	awsAccessKey  string
	awsSecretKey  string
	awsBucket     string
	awsRegion     = "eu-west-1"
	localRootPath string

	downloadConcurrency = 8
	fileConcurrency     = 1
)

func listBucket() {
	bucket := newBucketConnection()
	marker := ""

	for {
		sourceList, err := bucket.List("", "", marker, 1000)
		if err != nil {
			log.Fatal(err)
		}

		for i := 0; i < len(sourceList.Contents); i++ {
			key := sourceList.Contents[i]
			fileQueue <- &key
		}

		if !sourceList.IsTruncated {
			break
		}

		lastIndex := len(sourceList.Contents) - 1
		lastKey := sourceList.Contents[lastIndex]
		marker = lastKey.Key
	}
}

func fileWorker() {
	for {
		key := <-fileQueue
		if key == nil {
			fileWaitGroup.Done()
			break
		}

		checkKey(key)
	}
}

func downloadWorker() {
	bucket := newBucketConnection()

	for {
		key := <-downloadQueue
		if key == nil {
			downloadWaitGroup.Done()
			break
		}

		downloadKey(bucket, key)
	}
}

func checkKey(key *s3.Key) {
	defer atomic.AddUint64(&totalFiles, 1)
	defer atomic.AddUint64(&totalBytes, uint64(key.Size))

	path := pathForKey(key)

	if checkFilePresence(path) {
		size := fileSize(path)

		if size == key.Size {
			log.Printf("Skipped %v", key.Key)
			return
		}

		log.Printf("Mismatched size for %v (expecting %d bytes, got %d bytes)", key.Key, key.Size, size)
	}

	downloadQueue <- key
}

func downloadKey(bucket *s3.Bucket, key *s3.Key) {
	path := pathForKey(key)

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

	bucketReader, err := bucket.GetReader(key.Key)
	defer bucketReader.Close()
	if err != nil {
		log.Fatal(err)
	}

	bytes, err := io.Copy(fileWriter, bucketReader)
	if err != nil {
		log.Fatal(err)
	}

	atomic.AddUint64(&syncedBytes, uint64(bytes))
	atomic.AddUint64(&syncedFiles, 1)

	log.Printf("Fetched %v (%d bytes)", key.Key, bytes)
}

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

func pathForKey(key *s3.Key) string {
	return filepath.Join(localRootPath, key.Key)
}

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

	fileQueue = make(chan *s3.Key, 10000)
	downloadQueue = make(chan *s3.Key, 10000)

	log.Printf("Starting bucket sync from %v to %v", awsBucket, localRootPath)

	startWorkers()
	listBucket()
	stopWorkers()

	log.Printf("Synced %v/%v seen files, updated %v/%v bytes", syncedFiles, totalFiles, syncedBytes, totalBytes)
}

func startWorkers() {
	for i := 0; i < fileConcurrency; i++ {
		fileWaitGroup.Add(1)
		go fileWorker()
	}

	for i := 0; i < downloadConcurrency; i++ {
		downloadWaitGroup.Add(1)
		go downloadWorker()
	}
}

func stopWorkers() {
	// We shutdown the workers by sending them a nil
	for i := 0; i < fileConcurrency; i++ {
		fileQueue <- nil
	}

	fileWaitGroup.Wait()

	for i := 0; i < downloadConcurrency; i++ {
		downloadQueue <- nil
	}

	downloadWaitGroup.Wait()
}
