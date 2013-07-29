#!/bin/sh
package="github.com/newspaperclub/hank"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hank_linux_amd64 $package
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o hank_darwin_amd64 $package
