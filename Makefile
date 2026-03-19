.PHONY: build run-dev run-prod test lint

NODE_BIN ?= $(shell command -v node 2>/dev/null)
export PATH := $(dir $(NODE_BIN)):$(PATH)

build:
	cd web && npm ci && npm run build
	go build -o bin/go-ftpes ./cmd/server

run-dev:
	@echo "Starting dev servers..."
	cd web && npm run dev &
	DEV_MODE=true go run ./cmd/server

run-prod: build
	./bin/go-ftpes

test:
	go test ./...

lint:
	golangci-lint run
