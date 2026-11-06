SHELL := /bin/bash
.PHONY: build test vet fmt all clean

BINARY := git-grab
DIST   := $(CURDIR)/dist
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty)

PLATFORMS := \
	darwin-amd64 darwin-arm64 \
	freebsd-386 freebsd-amd64 freebsd-arm \
	linux-386 linux-amd64 linux-arm linux-arm64 \
	openbsd-386 openbsd-amd64 \
	windows-386 windows-amd64

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -cover ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

all: clean test vet
	mkdir -p $(DIST)
	for p in $(PLATFORMS); do \
		echo "Building $$p"; \
		ext=""; [[ $$p == windows-* ]] && ext=".exe"; \
		CGO_ENABLED=0 GOOS=$${p%-*} GOARCH=$${p#*-} \
		go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-$$p$$ext . || exit 1; \
	done
	ls -al $(DIST)

clean:
	rm -rf $(DIST) $(BINARY)
