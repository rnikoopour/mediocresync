.PHONY: build dev test lint

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
