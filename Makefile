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
	go get -v; \
	go build -v -ldflags -s -o $(DIST)/git-grab-test; \
	ls -al $(DIST) \

all:
	rm -rf $(DIST); \
	mkdir -p $(DIST); \
	go get -d; \
	GOOS=darwin-amd64 GOARCH=darwin-amd64 go build -v -ldflags -s -o $(DIST)/git-grab-darwin-amd64; \
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
		GOOS=$${p/-*/} GOARCH=$${p/*-/} go build -v -ldflags -s -o $(DIST)/git-grab-$$p; \
   	done
