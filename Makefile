.PHONY: run dev build test clean migrate-up migrate-down seed sqlc docker-up docker-down swagger

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

BINARY_NAME=logistics-api
BINARY_UNIX=$(BINARY_NAME)_unix

DB_URL=postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable

build:
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/api

run:
	$(GOCMD) run ./cmd/api

dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air -c .air.toml

test:
	$(GOTEST) -v ./...

test-integration:
	$(GOTEST) -v -tags=integration ./tests/integration/...

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
	sqlc generate

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

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) ./cmd/api