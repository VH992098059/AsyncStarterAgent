.PHONY: help build run test test-integration lint migrate-up migrate-down docker-up docker-down clean

GO ?= go
APP := bin/api
MIGRATE := github.com/golang-migrate/migrate/v4/cmd/migrate@latest
DATABASE_URL ?= postgres://starter:starter@localhost:5432/starter?sslmode=disable

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

migrate-up: ## 应用迁移
	$(GO) run -tags 'postgres' $(MIGRATE) -database "$(DATABASE_URL)" -path ./migrations up

migrate-down: ## 回滚迁移
	$(GO) run -tags 'postgres' $(MIGRATE) -database "$(DATABASE_URL)" -path ./migrations down 1

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/ coverage.html coverage.txt
