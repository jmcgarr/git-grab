# 01: Toolchain, Makefile, and CI

## Problem

- `go.mod` declares `go 1.22`. Go 1.22.0 produces binaries that abort on
  macOS 26 with `missing LC_UUID load command`. Tests cannot run locally.
  Verified: the same code passes under Go 1.26.0.
- The Makefile uses bash-only `${p/-*/}` expansion. Ubuntu's `/bin/sh` is dash,
  which is why CI was moved to a macOS runner. `make build` runs `go get -v`
  and builds nothing. `go get -d` is deprecated. No `.PHONY` targets.
- GitHub reports zero workflow runs. The pipeline has never provided signal.

## Changes

### go.mod

```
module github.com/jmcgarr/git-grab

go 1.26
```

The module path change is what makes `go install github.com/jmcgarr/git-grab@latest`
work. It has no other effect on a single-package module.

### Makefile

Replace the whole file with:

```make
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
```

Notes:
- `linux-arm64` is added. Raspberry Pi and ARM servers are common now.
- Windows binaries get `.exe` so they can be run directly.
- `-X main.version` feeds the `--version` flag added in spec 03. Until that
  lands the flag is harmless.
- Add `git-grab` and `*.exe` to `.gitignore`.
- `|| exit 1` makes the loop stop at the first failed platform. Without it
  the recipe's exit status is that of the last iteration only, so a broken
  port would pass CI and ship a release with missing binaries.

### CI: `.github/workflows/ci.yml`

Rename `makefile.yml` to `ci.yml` and replace its contents:

```yaml
name: CI

on:
  push:
    branches: [master]
    tags: ['*']
  pull_request:
    branches: [master]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make fmt
      - run: make vet
      - run: make test
      - run: make all
      - uses: actions/upload-artifact@v4
        with:
          name: dist
          path: dist/

  release:
    if: startsWith(github.ref, 'refs/tags/')
    needs: check
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: dist
          path: dist
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*
          fail_on_unmatched_files: true
          generate_release_notes: true
```

The release job replaces the manual upload of twelve binaries. Pushing a tag
builds, tests, and attaches the assets. Release notes can still be edited
afterwards on GitHub.

## Acceptance

- `go test ./...` passes locally on macOS 26 with the toolchain selected by
  `go.mod`.
- `make all` succeeds on both macOS and Ubuntu.
- A push to master shows a green run under the Actions tab.
- Pushing a test tag such as `0.5-rc1` produces a draft-free release with
  thirteen assets. Delete the tag and release afterwards.
