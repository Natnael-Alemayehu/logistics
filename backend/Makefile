.PHONY: help db db-stop run dev build migrate seed test test-integration vet sqlc docker-build clean sandbox sandbox-keys run-sandbox

DB_PORT ?= 5434
DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/logistics?sslmode=disable
# Integration tests create (and drop) a scratch database on this server.
TEST_DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/postgres?sslmode=disable
export DATABASE_URL
# Runs a command with sandbox.env's settings, for the app and the sandbox.
WITH_SANDBOX_ENV = set -a && . ./sandbox.env && set +a &&

help:
	@echo "Menged"
	@echo "  make db                 start the development database (port $(DB_PORT))"
	@echo "  make seed               migrate and load the demo company"
	@echo "  make run                run the app on :8080, templates reloaded from disk"
	@echo "  make dev                run with rebuild on Go changes (air)"
	@echo "  make sandbox            run the provider sandbox on :8091: SMS, telebirr, CBE Birr"
	@echo "  make run-sandbox        run the app with its providers pointed at the sandbox"
	@echo "  make test               unit tests"
	@echo "  make test-integration   end-to-end tests against the database"
	@echo "  make build              build bin/menged and bin/menged-sandbox"
	@echo "  make migrate            apply migrations"
	@echo "  make sqlc               regenerate internal/db from sqlc/queries"
	@echo "  make docker-build       build the production image and the sandbox's"

db:
	DB_PORT=$(DB_PORT) docker compose up -d --wait postgres

db-stop:
	docker compose down

run:
	DEV_RELOAD=true go run ./cmd/server serve

dev:
	@command -v air >/dev/null || go install github.com/air-verse/air@v1.62.0
	DEV_RELOAD=true air -c .air.toml

# The provider sandbox and the app pointed at it, in two terminals with the
# same PORT. The telebirr keys are made on the first run.
sandbox: sandbox-keys
	$(WITH_SANDBOX_ENV) go run ./cmd/sandbox

run-sandbox: sandbox-keys
	$(WITH_SANDBOX_ENV) DEV_RELOAD=true go run ./cmd/server serve

sandbox-keys:
	@go run ./cmd/sandbox keys

build:
	CGO_ENABLED=0 go build -trimpath -o bin/menged ./cmd/server
	CGO_ENABLED=0 go build -trimpath -o bin/menged-sandbox ./cmd/sandbox

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
	docker build --target sandbox -t menged-sandbox:latest .

clean:
	rm -rf bin tmp
