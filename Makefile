VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
PACKAGE := github.com/orkward/selever/pkg/config

LDFLAGS := -s -w \
	-X $(PACKAGE).Version=$(VERSION) \
	-X $(PACKAGE).GitCommit=$(GIT_COMMIT) \
	-X $(PACKAGE).BuildTime=$(BUILD_TIME)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o ./ ./cmd/*

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)"
