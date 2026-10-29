.PHONY: help db db-stop run dev build migrate seed test test-integration vet sqlc docker-build clean

DB_PORT ?= 5434
DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/logistics?sslmode=disable
# Integration tests create (and drop) a scratch database on this server.
TEST_DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/postgres?sslmode=disable
export DATABASE_URL

help:
	@echo "Menged"
	@echo "  make db                 start the development database (port $(DB_PORT))"
	@echo "  make seed               migrate and load the demo company"
	@echo "  make run                run the app on :8080, templates reloaded from disk"
	@echo "  make dev                run with rebuild on Go changes (air)"
	@echo "  make test               unit tests"
	@echo "  make test-integration   end-to-end tests against the database"
	@echo "  make build              build bin/menged"
	@echo "  make migrate            apply migrations"
	@echo "  make sqlc               regenerate internal/db from sqlc/queries"
	@echo "  make docker-build       build the production image"

db:
	DB_PORT=$(DB_PORT) docker compose up -d --wait postgres

db-stop:
	docker compose down

run:
	DEV_RELOAD=true go run ./cmd/server serve

dev:
	@command -v air >/dev/null || go install github.com/air-verse/air@v1.62.0
	DEV_RELOAD=true air -c .air.toml

build:
	CGO_ENABLED=0 go build -trimpath -o bin/menged ./cmd/server

migrate:
	go run ./cmd/server migrate

seed:
	go run ./cmd/server seed

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -tags integration -count=1 ./tests/integration/

vet:
	go vet ./...
	go vet -tags integration ./tests/...

sqlc:
	sqlc generate -f sqlc/sqlc.yaml

docker-build:
	docker build -t menged:latest .

clean:
	rm -rf bin tmp
