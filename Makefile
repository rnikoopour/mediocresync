.PHONY: build run-dev run-prod test lint

NODE_BIN ?= $(shell command -v node 2>/dev/null)
export PATH := $(dir $(NODE_BIN)):$(PATH)

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY := bin/mediocresync-$(GOOS)-$(GOARCH)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	cd web && npm ci && npm run build
	touch ui/dist/.gitkeep
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/server

LISTEN_ALL ?= false
VITE_HOST_FLAG := $(if $(filter true,$(LISTEN_ALL)),-- --host,)

run-dev:
	@echo "Starting dev servers..."
	cd web && npm run dev $(VITE_HOST_FLAG) &
	DEV_MODE=true go run ./cmd/server

run-prod: build
	./$(BINARY)

test:
	@# Stash ui/dist so stray files (e.g. .go files from node_modules copied by vite)
	@# don't get picked up by go test ./..., then restore it when done.
	@DIST_BACKUP=$$(mktemp -d) && \
	  mv ui/dist $$DIST_BACKUP/ && \
	  mkdir -p ui/dist && touch ui/dist/.gitkeep && \
	  go test ./... ; GO_EXIT=$$? ; \
	  rm -rf ui/dist && mv $$DIST_BACKUP/dist ui/dist && \
	  exit $$GO_EXIT
	cd web && npm test -- --run

lint:
	golangci-lint run
