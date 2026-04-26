BINARY ?= foundry
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/AES-Services/foundry-cli/internal/version.Version=$(VERSION) -X github.com/AES-Services/foundry-cli/internal/version.Commit=$(COMMIT) -X github.com/AES-Services/foundry-cli/internal/version.Date=$(DATE)

.PHONY: build test ci

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/foundry

test:
	go test ./...

ci: test build
