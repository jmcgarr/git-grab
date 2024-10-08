PLATFORMS= \
	darwin-amd64 \
	darwin-arm64 \
	freebsd-386 \
	freebsd-amd64 \
	freebsd-arm \
	linux-386 \
	linux-amd64 \
	linux-arm \
	openbsd-386 \
	openbsd-amd64 \
	windows-386 \
	windows-amd64 \
	$(NULL)

DIST=$(shell pwd)/dist

build:
	go get -v;

test:
	go get -v; \
	go test;

all:
	rm -rf $(DIST); \
	mkdir -p $(DIST); \
	go get -d; \
	go test; \
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
	    GOOS=$${p/-*/} GOARCH=$${p/*-/} go build -v -ldflags -s -o $(DIST)/git-grab-$$p; \
   	done 
	ls -al $(DIST) \