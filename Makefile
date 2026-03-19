.PHONY: build dev test lint

NODE_BIN ?= $(shell command -v node 2>/dev/null)
export PATH := $(dir $(NODE_BIN)):$(PATH)

build:
	cd web && npm ci && npm run build
	go build -o bin/go-ftpes ./cmd/server

dev:
	@echo "Starting dev servers..."
	cd web && npm run dev &
	go run ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run
