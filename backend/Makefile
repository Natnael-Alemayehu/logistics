.PHONY: help run dev build test clean migrate-up migrate-down seed sqlc docker-up docker-down swagger deps

help:
	@echo "Backend - Logistics API Server"
	@echo "==============================="
	@echo "  make deps        - Download Go dependencies"
	@echo "  make dev         - Start development server with hot reload"
	@echo "  make build       - Build API and worker binaries"
	@echo "  make run         - Run API server"
	@echo "  make run-worker  - Run worker"
	@echo "  make test        - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make migrate-up  - Run database migrations"
	@echo "  make migrate-down - Rollback database migrations"
	@echo "  make seed        - Seed database with test data"
	@echo "  make sqlc        - Generate SQLC code"
	@echo "  make swagger     - Generate Swagger docs"
	@echo "  make docker-up   - Start Docker containers"
	@echo "  make docker-down - Stop Docker containers"
	@echo "  make setup       - Full development setup"

deps:
	go mod download

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

BINARY_NAME=logistics-api
WORKER_BINARY=logistics-worker
BINARY_UNIX=$(BINARY_NAME)_unix

# Database
DB_URL=postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable

# Main build targets
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/api
	$(GOBUILD) -o bin/$(WORKER_BINARY) ./cmd/worker

run:
	$(GOCMD) run ./cmd/api

run-worker:
	$(GOCMD) run ./cmd/worker

dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air -c .air.toml

test:
	$(GOTEST) -v -short ./...

# Integration tests with configurable timeout (default: 30m)
# Usage: make test-integration TIMEOUT=1h
TIMEOUT ?= 30m
test-integration:
	$(GOTEST) -v -tags=integration -timeout $(TIMEOUT) ./tests/integration/...

test-all:
	$(GOTEST) -v -short ./...
	$(GOTEST) -v -tags=integration -timeout $(TIMEOUT) ./tests/integration/...

ci: test-all

clean:
	$(GOCLEAN)
	rm -rf bin/

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

seed:
	@echo "Seeding development database..."
	$(GOCMD) run scripts/seed.go "$(DB_URL)"

migrate-create:
	@read -p "Enter migration name: " name; \
	goose -dir migrations create "$$name" sql

sqlc:
	cd sqlc && sqlc generate

swagger:
	swag init -g cmd/api/main.go -o docs

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api

setup: docker-up sleep migrate-up
	@echo "Development environment ready!"

sleep:
	sleep 5

pgcli:
	pgcli "postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable"

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) ./cmd/api
