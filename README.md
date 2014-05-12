# Hank

Quick and dirty script to backup an S3 bucket to a local disk, written in Go.
[s3sync](http://s3sync.net/wiki) is great, but routinely runs out of memory
when downloading large buckets and is terminated by the OOM killer.

We use hank as part of our backup tools to sync a ~400GB S3 bucket, containing
~700,000 files to a reasonably slow local filesystem in approximately
6 minutes.

It doesn't check file contents and will skip anything that already exists
locally, assuming the file size matches. Only cares about objects (files), and
won't attempt to replicate directories. Will delete files locally that have
been deleted remotely, but won't clean up any directories that were created by
those.

Listing the bucket and iterating over the filesystem uses a single goroutine
each, but downloading uses 8 simultaneously to maximise network throughput.

The [releases page on GitHub](https://github.com/newspaperclub/hank/releases)
contains statically compiled executables for linux/amd64 and darwin/amd64, but
I'm happy to add other architectures to the build script if needed.

## Usage

    hank --access-key FOO --secret-key BAR --region eu-west-1 --bucket bucket-name --download-concurrency=8 path/to/destination
