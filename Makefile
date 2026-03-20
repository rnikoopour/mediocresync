.PHONY: build run-dev run-prod test lint

NODE_BIN ?= $(shell command -v node 2>/dev/null)
export PATH := $(dir $(NODE_BIN)):$(PATH)

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

build:
	cd web && npm ci && npm run build
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/mediocresync ./cmd/server

run-dev:
	@echo "Starting dev servers..."
	cd web && npm run dev &
	DEV_MODE=true go run ./cmd/server

run-prod: build
	./bin/mediocresync

test:
	go test ./...

lint:
	golangci-lint run
