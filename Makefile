PACKAGE=github.com/newspaperclub/hank

all: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hank_linux_amd64 $(PACKAGE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o hank_darwin_amd64 $(PACKAGE)

clean:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go clean $(PACKAGE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go clean $(PACKAGE)

