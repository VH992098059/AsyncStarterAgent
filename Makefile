.PHONY: help build run test test-integration lint migrate-up migrate-down docker-up docker-down clean

GO ?= go
APP := bin/api

help:
	@echo "Available targets:"
	@echo "  build              compile binary"
	@echo "  run                run locally"
	@echo "  test               unit tests"
	@echo "  test-integration   integration tests (needs docker compose up)"
	@echo "  lint               golangci-lint"
	@echo "  migrate-up         apply database migrations (T003+)"
	@echo "  migrate-down       rollback one migration (T003+)"
	@echo "  docker-up          start docker compose"
	@echo "  docker-down        stop docker compose"
	@echo "  clean              remove build artifacts"

build:
	$(GO) build -o $(APP) ./cmd/api

run:
	$(GO) run ./cmd/api

test:
	$(GO) test -race -short ./...

test-integration:
	$(GO) test -race -tags=integration ./test/integration/...

lint:
	golangci-lint run ./...

migrate-up:
	@echo "TODO: integrate golang-migrate in T003"

migrate-down:
	@echo "TODO: integrate golang-migrate in T003"

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/ coverage.html coverage.txt
