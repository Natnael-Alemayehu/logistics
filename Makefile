.PHONY: help db db-stop run dev build migrate seed test test-integration test-integration-cover test-e2e vet lint sqlc docker-build clean \
	sandbox sandbox-keys run-sandbox \
	staging-up staging-down staging-ca staging-logs

DB_PORT ?= 5434
DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/logistics?sslmode=disable
# Integration tests create (and drop) a scratch database on this server.
TEST_DATABASE_URL ?= postgres://logistics:logistics@localhost:$(DB_PORT)/postgres?sslmode=disable
export DATABASE_URL
# Development: the database, and the sandbox in a container if wanted.
DEV = docker compose -f deployments/dev/compose.yml
# Runs a command with sandbox.env's settings, for the app and the sandbox.
WITH_SANDBOX_ENV = set -a && . deployments/dev/sandbox.env && set +a &&
# Staging: production's compose file, with the provider sandbox for the
# providers (deployments/staging/compose.yml).
STAGING = docker compose -f deployments/docker-compose.prod.yml -f deployments/staging/compose.yml \
	--env-file deployments/staging/staging.env
STAGING_CA = tmp/staging-ca.crt
# The linters `make lint` runs. Dependabot doesn't see these.
GOLANGCI_LINT_VERSION = v2.15.0
GITLEAKS_VERSION = v8.30.1

help:
	@echo "Kiflet"
	@echo "  make db                 start the development database (port $(DB_PORT))"
	@echo "  make seed               migrate and load the demo company"
	@echo "  make run                run the app on :8080, templates reloaded from disk"
	@echo "  make dev                run with rebuild on Go changes (air)"
	@echo "  make sandbox            run the provider sandbox on :8091: SMS, telebirr, CBE Birr"
	@echo "  make run-sandbox        run the app with its providers pointed at the sandbox"
	@echo "  make test               unit tests"
	@echo "  make test-integration   end-to-end tests against the database"
	@echo "  make test-integration-cover   the same, with what they cover of the service and web packages"
	@echo "  make staging-up         run staging at https://kiflet.localhost:8443"
	@echo "  make test-e2e           end-to-end tests against staging"
	@echo "  make staging-down       remove staging and its data"
	@echo "  make build              build bin/kiflet and bin/kiflet-sandbox"
	@echo "  make migrate            apply migrations"
	@echo "  make vet                go vet, the tests included"
	@echo "  make lint               golangci-lint, the JavaScript parsed, and a secret scan of the history"
	@echo "  make sqlc               regenerate internal/db from sqlc/queries"
	@echo "  make docker-build       build the production image and the sandbox's"

db:
	DB_PORT=$(DB_PORT) $(DEV) up -d --wait postgres

db-stop:
	$(DEV) down

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
	CGO_ENABLED=0 go build -trimpath -o bin/kiflet ./cmd/server
	CGO_ENABLED=0 go build -trimpath -o bin/kiflet-sandbox ./cmd/sandbox

migrate:
	go run ./cmd/server migrate

seed:
	go run ./cmd/server seed

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -tags integration -count=1 ./tests/integration/

# The same, measuring what they cover of internal/service and internal/web,
# whose tests they are: coverage.out, and a line per package. CI runs this.
test-integration-cover:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -tags integration -count=1 \
		-coverpkg=./internal/service,./internal/web -coverprofile=coverage.out ./tests/integration/
	@scripts/coverage-summary.sh coverage.out

# Against staging (make staging-up), through Caddy as a browser and the
# providers reach it.
test-e2e:
	E2E_CA_FILE=$(CURDIR)/$(STAGING_CA) go test -tags e2e -count=1 -v ./tests/e2e/

staging-up:
	$(STAGING) up -d --build --wait --wait-timeout 600
	@$(MAKE) --no-print-directory staging-ca
	@echo "Staging is up: https://kiflet.localhost:8443, the sandbox at https://sandbox.kiflet.localhost:8443"
	@echo "and email at https://mail.kiflet.localhost:8443. Caddy's root certificate is $(STAGING_CA)."

staging-down:
	$(STAGING) down --volumes --remove-orphans

# Caddy's root certificate, for the tests, curl or a browser to trust.
staging-ca:
	@mkdir -p $(dir $(STAGING_CA))
	@$(STAGING) cp caddy:/data/caddy/pki/authorities/local/root.crt $(STAGING_CA)

staging-logs:
	$(STAGING) logs --follow --tail=100

vet:
	go vet ./...
	go vet -tags integration ./tests/...
	go vet -tags e2e ./tests/...

# golangci-lint over the Go (.golangci.yml); Node parsing each script of our
# own, which browsers run as written, with no build step to catch a slip; and
# gitleaks over the whole history (.gitleaks.toml).
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run
	@for f in $$(git ls-files '*.js' ':!:*/vendor/*'); do node --check "$$f" || exit 1; done
	go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION) git --redact --no-banner .

sqlc:
	sqlc generate -f sqlc/sqlc.yaml

docker-build:
	docker build -t kiflet:latest .
	docker build --target sandbox -t kiflet-sandbox:latest .

clean:
	rm -rf bin tmp
