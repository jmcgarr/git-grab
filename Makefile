PLATFORMS= \
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
	darwin-386 \
	darwin-amd64 \
	$(NULL)

DIST=$(shell pwd)/dist
export GOPATH=$(shell pwd)

build:
	cd src/github.com/jmcgarr/git-grab; \
	go get -v

all:
	rm -rf $(DIST); \
	mkdir -p $(DIST); \
	cd src/github.com/jmcgarr/git-grab; \
	go get -d; \
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
		GOOS=$${p/-*/} GOARCH=$${p/*-/} go build -v -ldflags -s -o $(DIST)/git-grab-$$p; \
   done