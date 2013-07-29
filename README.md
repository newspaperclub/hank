# Hank

Quick and dirty script to backup an S3 bucket to a local disk, written in Go.

It doesn't check file contents and will skip anything that already exists
locally.

Uses 8 workers to download and process files simultaneously.

## Usage

    hank --access-key FOO --secret-key BAR --region eu-west-1 --bucket bucket-name path/to/destination
