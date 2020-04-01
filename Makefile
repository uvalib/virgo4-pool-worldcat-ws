GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOFMT = $(GOCMD) fmt
GOVET = $(GOCMD) vet
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

build: darwin 

all: darwin linux

darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -a -o bin/v4worldcat.darwin cmd/*.go
	cp -r i18n/ bin/i18n
	cp -r assets/ bin/assets

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -o bin/v4worldcat.linux cmd/*.go
	cp -r i18n/ bin/i18n
	cp -r assets/ bin/assets

clean:
	$(GOCLEAN) cmd/
	rm -rf bin

fmt:
	cd cmd; $(GOFMT)

vet:
	cd cmd; $(GOVET)

dep:
	$(GOGET) -u ./cmd/...
	$(GOMOD) tidy
	$(GOMOD) verify

check:
	go get honnef.co/go/tools/cmd/staticcheck
	~/go/bin/staticcheck -checks all,-S1002,-ST1003 cmd/*.go
