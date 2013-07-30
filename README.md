# Hank

Quick and dirty script to backup an S3 bucket to a local disk, written in Go.
[s3sync](http://s3sync.net/wiki) is great, but routinely runs out of memory
when downloading large buckets and is terminated by the OOM killer.

We use hank as part of our backup tools to sync a ~400GB S3 bucket to the local
filesystem in approximately 6 minutes, most of which is disk IO wait.

It doesn't check file contents and will skip anything that already exists
locally. Only cares about objects (files), and won't attempt to replicate
directories. Doesn't delete files locally that have been deleted remotely.

It walks the bucket using a single goroutine, queuing the found keys to
a channel. Multiple goroutines (8 of them) then pull keys off the channel,
check whether they exist locally, and if not, fetches them.

## Usage

    hank --access-key FOO --secret-key BAR --region eu-west-1 --bucket bucket-name path/to/destination
