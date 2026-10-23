.PHONY: help install dev-api dev-web build lint lint-go lint-web lint-docs format format-check typecheck test test-race check

help:
	@echo "install       Install pinned Node dependencies"
	@echo "dev-api       Run the Go skeleton server (long-running)"
	@echo "dev-web       Run the Vite development server (long-running)"
	@echo "build         Build the Go binary and frontend"
	@echo "lint          Check Go, frontend, docs, and formatting"
	@echo "test          Run Go tests and frontend type checks"
	@echo "test-race     Run Go tests with the race detector"
	@echo "format        Format Go, frontend, and documentation"
	@echo "check         Run lint, test, and build"

install:
	npm ci

dev-api:
	go run ./cmd/watchledger

dev-web:
	npm run dev

build:
	mkdir -p build
	go build -trimpath -o build/watchledger ./cmd/watchledger
	npm run build

lint: lint-go lint-web lint-docs format-check

lint-go:
	golangci-lint run ./cmd/... ./internal/...

lint-web:
	npm run lint:web

lint-docs:
	npm run lint:docs

format:
	gofmt -w cmd internal
	npm run format

format-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
	npm run format:check

typecheck:
	npm run typecheck

test:
	go test ./cmd/... ./internal/...
	npm run typecheck

test-race:
	go test -race ./cmd/... ./internal/...

check: lint test build
