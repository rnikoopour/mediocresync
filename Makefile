.PHONY: build run-dev run-prod test lint

NODE_BIN ?= $(shell command -v node 2>/dev/null)
export PATH := $(dir $(NODE_BIN)):$(PATH)

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

build:
	cd web && npm ci && npm run build
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/mediocresync ./cmd/server

LISTEN_ALL ?= false
VITE_HOST_FLAG := $(if $(filter true,$(LISTEN_ALL)),-- --host,)

run-dev:
	@echo "Starting dev servers..."
	cd web && npm run dev $(VITE_HOST_FLAG) &
	DEV_MODE=true go run ./cmd/server

run-prod: build
	./bin/mediocresync

test:
	# web/node_modules contains Go files that confuse ./...; list packages explicitly instead
	go test ./cmd/... ./internal/... ./ui/...
	cd web && npm test -- --run

lint:
	golangci-lint run
