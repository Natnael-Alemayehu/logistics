# Backend Development Plan
## Ethiopian Logistics Tracking Platform

**Language**: Go (Golang)  
**Router**: chi  
**Migrations**: goose  
**Database**: PostgreSQL + PostGIS  
**Query Builder**: sqlc  
**Testing**: testcontainers  

---

## Project Structure

```
logistics/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Environment configuration
│   ├── db/
│   │   ├── db.go                   # Database connection
│   │   ├── models.go               # sqlc generated models
│   │   ├── querier.go              # sqlc generated interface
│   │   └── queries.sql.go          # sqlc generated queries
│   ├── handler/
│   │   ├── handler.go              # Handler dependencies
│   │   ├── auth.go                 # Authentication handlers
│   │   ├── shipment.go             # Shipment handlers
│   │   ├── tracking.go             # Tracking event handlers
│   │   ├── pod.go                  # Proof of delivery handlers
│   │   ├── user.go                 # User management handlers
│   │   ├── sync.go                 # Sync protocol handlers
│   │   └── health.go               # Health check handlers
│   ├── middleware/
│   │   ├── auth.go                 # JWT authentication
│   │   ├── tenant.go               # Multi-tenant context
│   │   ├── logging.go              # Request logging
│   │   ├── cors.go                 # CORS configuration
│   │   └── recovery.go             # Panic recovery
│   ├── service/
│   │   ├── service.go              # Service interfaces
│   │   ├── auth.go                 # Auth business logic
│   │   ├── shipment.go             # Shipment business logic
│   │   ├── tracking.go             # Tracking business logic
│   │   ├── pod.go                  # POD business logic
│   │   ├── sync.go                 # Sync business logic
│   │   └── user.go                 # User business logic
│   ├── repository/
│   │   ├── repository.go           # Repository interfaces
│   │   ├── user.go                 # User data access
│   │   ├── shipment.go             # Shipment data access
│   │   ├── tracking.go             # Tracking data access
│   │   ├── pod.go                  # POD data access
│   │   └── session.go              # Session data access
│   └── model/
│       ├── user.go                 # User domain model
│       ├── shipment.go             # Shipment domain model
│       ├── tracking.go             # Tracking event domain model
│       ├── pod.go                  # POD domain model
│       ├── sync.go                 # Sync request/response models
│       └── errors.go               # Domain errors
├── pkg/
│   ├── jwt/
│   │   ├── jwt.go                  # JWT generation/validation
│   │   └── keys.go                 # RSA key management
│   ├── validation/
│   │   └── validation.go           # Validator setup
│   ├── response/
│   │   └── response.go             # JSON response utilities
│   └── hash/
│       └── hash.go                 # Password/PIN hashing
├── migrations/
│   ├── 20260216000000_initial_schema.up.sql
│   ├── 20260216000000_initial_schema.down.sql
│   └── ... additional migrations
├── sqlc/
│   ├── queries/
│   │   ├── users.sql               # User queries
│   │   ├── shipments.sql           # Shipment queries
│   │   ├── tracking.sql            # Tracking queries
│   │   ├── pod.sql                 # POD queries
│   │   └── sessions.sql            # Session queries
│   └── sqlc.yaml                   # sqlc configuration
├── tests/
│   ├── integration/
│   │   ├── main_test.go            # Test setup
│   │   ├── auth_test.go            # Auth integration tests
│   │   ├── shipment_test.go        # Shipment integration tests
│   │   └── sync_test.go            # Sync integration tests
│   └── fixtures/
│       └── fixtures.go             # Test data fixtures
├── docs/
│   └── api/
│       └── openapi.yaml            # OpenAPI specification
├── scripts/
│   ├── generate_keys.sh            # Generate RSA key pair
│   └── seed_dev.sh                 # Seed development data
├── docker-compose.yml
├── docker-compose.test.yml
├── Dockerfile
├── Dockerfile.test
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Phase 1: Project Foundation

### Step 1.1: Initialize Go Module

```bash
cd /home/nate/Projects/logistics
go mod init github.com/yourusername/logistics
```

### Step 1.2: Create Directory Structure

```bash
mkdir -p cmd/api
mkdir -p internal/{config,db,handler,middleware,service,repository,model}
mkdir -p pkg/{jwt,validation,response,hash}
mkdir -p migrations
mkdir -p sqlc/queries
mkdir -p tests/{integration,fixtures}
mkdir -p docs/api
mkdir -p scripts
```

### Step 1.3: Install Dependencies

```bash
# Router
go get github.com/go-chi/chi/v5
go get github.com/go-chi/chi/v5/middleware
go get github.com/go-chi/cors

# Database
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool

# Migrations
go get github.com/pressly/goose/v3

# Validation
go get github.com/go-playground/validator/v10

# JWT
go get github.com/golang-jwt/jwt/v5

# Logging
go get github.com/rs/zerolog

# Testing
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres

# Password hashing
go get golang.org/x/crypto/bcrypt

# Configuration (minimal)
go get github.com/joho/godotenv
```

### Step 1.4: Create Makefile

```makefile
.PHONY: run build test clean migrate-up migrate-down sqlc docker-up docker-down

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=logistics-api
BINARY_UNIX=$(BINARY_NAME)_unix

# Database
DB_URL=postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable

# Main build targets
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/api

run:
	$(GOCMD) run ./cmd/api

test:
	$(GOTEST) -v ./...

test-integration:
	$(GOTEST) -v -tags=integration ./tests/integration/...

clean:
	$(GOCLEAN)
	rm -rf bin/

# Database migrations
migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-create:
	@read -p "Enter migration name: " name; \
	goose -dir migrations create "$$name" sql

# sqlc code generation
sqlc:
	sqlc generate

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f api

# Development setup
setup: docker-up sleep migrate-up
	@echo "Development environment ready!"

sleep:
	sleep 5

# Build for production
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) ./cmd/api
```

### Step 1.5: Create docker-compose.yml

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    container_name: logistics-db
    environment:
      POSTGRES_USER: logistics
      POSTGRES_PASSWORD: logistics
      POSTGRES_DB: logistics
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U logistics"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: logistics-api
    environment:
      - DATABASE_URL=postgres://logistics:logistics@postgres:5432/logistics?sslmode=disable
      - JWT_PRIVATE_KEY_PATH=/app/keys/private.pem
      - JWT_PUBLIC_KEY_PATH=/app/keys/public.pem
      - JWT_ACCESS_TTL=3600
      - JWT_REFRESH_TTL=2592000
      - LOG_LEVEL=debug
      - PORT=8080
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./migrations:/app/migrations
      - ./keys:/app/keys

volumes:
  postgres_data:
```

### Step 1.6: Create Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

RUN mkdir -p /app/keys

EXPOSE 8080

CMD ["./main"]
```

---

## Phase 2: Database Schema & Migrations

### Step 2.1: Initial Migration - Core Tables

File: `migrations/20260216000000_initial_schema.up.sql`

```sql
-- Enable PostGIS extension for spatial queries
CREATE EXTENSION IF NOT EXISTS postgis;

-- Tenants (Companies)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    plan TEXT NOT NULL DEFAULT 'starter',
    max_drivers INTEGER DEFAULT 5,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Users (Drivers, Dispatchers, Admins)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('driver', 'dispatcher', 'fleet_manager', 'admin', 'platform_admin')),
    full_name TEXT NOT NULL,
    phone TEXT,
    email TEXT,
    password_hash TEXT,
    pin_hash TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, phone),
    UNIQUE(tenant_id, email),
    UNIQUE(phone) WHERE phone IS NOT NULL,
    UNIQUE(email) WHERE email IS NOT NULL
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_users_phone ON users(phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

-- Vehicles
CREATE TABLE vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plate_number TEXT NOT NULL,
    vehicle_type TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, plate_number)
);

CREATE INDEX idx_vehicles_tenant ON vehicles(tenant_id);

-- Shipments
CREATE TABLE shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    tracking_number TEXT UNIQUE NOT NULL,
    
    -- Origin
    origin_address TEXT NOT NULL,
    origin_coordinates GEOMETRY(POINT, 4326),
    
    -- Destination
    destination_address TEXT NOT NULL,
    destination_coordinates GEOMETRY(POINT, 4326),
    
    -- Customer
    customer_name TEXT NOT NULL,
    customer_phone TEXT NOT NULL,
    
    -- Cargo
    cargo_description TEXT,
    cargo_weight DECIMAL(10, 2),
    cargo_value DECIMAL(15, 2),
    special_instructions TEXT,
    
    -- Assignment
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    vehicle_id UUID REFERENCES vehicles(id) ON DELETE SET NULL,
    
    -- Status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_transit', 'delayed', 'arrived', 'delivered', 'issue', 'cancelled')),
    status_note TEXT,
    status_reason TEXT,
    
    -- Timing
    estimated_delivery TIMESTAMPTZ,
    actual_delivery TIMESTAMPTZ,
    
    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID REFERENCES users(id)
);

CREATE INDEX idx_shipments_tenant ON shipments(tenant_id);
CREATE INDEX idx_shipments_tracking ON shipments(tracking_number);
CREATE INDEX idx_shipments_driver ON shipments(driver_id);
CREATE INDEX idx_shipments_status ON shipments(status);
CREATE INDEX idx_shipments_created ON shipments(created_at);
CREATE INDEX idx_shipments_destination_coords ON shipments USING GIST(destination_coordinates);

-- Tracking Events (GPS pings, status changes, checkpoints)
CREATE TABLE tracking_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Location
    coordinates GEOMETRY(POINT, 4326) NOT NULL,
    accuracy_meters DECIMAL(10, 2),
    speed_kph DECIMAL(10, 2),
    heading DECIMAL(5, 2),
    
    -- Event details
    event_type TEXT NOT NULL CHECK (event_type IN ('gps_ping', 'checkpoint', 'status_change')),
    status TEXT,
    note TEXT,
    
    -- Timestamps
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ,
    
    -- Device info
    device_id TEXT,
    battery_level INTEGER
);

CREATE INDEX idx_tracking_tenant ON tracking_events(tenant_id);
CREATE INDEX idx_tracking_shipment ON tracking_events(shipment_id);
CREATE INDEX idx_tracking_driver ON tracking_events(driver_id);
CREATE INDEX idx_tracking_recorded ON tracking_events(recorded_at);
CREATE INDEX idx_tracking_coords ON tracking_events USING GIST(coordinates);

-- Proof of Delivery
CREATE TABLE proof_of_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID UNIQUE NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    driver_id UUID NOT NULL REFERENCES users(id),
    
    -- Recipient
    recipient_name TEXT NOT NULL,
    recipient_phone TEXT,
    
    -- Evidence
    signature_data TEXT,
    signature_url TEXT,
    photo_urls TEXT[],
    
    -- Location
    delivery_address TEXT,
    delivery_coordinates GEOMETRY(POINT, 4326),
    
    -- Notes
    delivery_notes TEXT,
    location_verified BOOLEAN DEFAULT false,
    location_mismatch_meters DECIMAL(10, 2),
    
    -- Timestamps
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ
);

CREATE INDEX idx_pod_tenant ON proof_of_deliveries(tenant_id);
CREATE INDEX idx_pod_shipment ON proof_of_deliveries(shipment_id);
CREATE INDEX idx_pod_driver ON proof_of_deliveries(driver_id);

-- Sessions (Refresh tokens)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    device_id TEXT,
    user_agent TEXT,
    ip_address TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Audit Logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    old_value JSONB,
    new_value JSONB,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant ON audit_logs(tenant_id);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

-- Sync Metadata (for delta sync)
CREATE TABLE sync_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_sync_at TIMESTAMPTZ,
    last_event_id UUID,
    pending_sync_count INTEGER DEFAULT 0,
    storage_used_kb INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Function to auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to tables with updated_at
CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vehicles_updated_at BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_shipments_updated_at BEFORE UPDATE ON shipments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sync_metadata_updated_at BEFORE UPDATE ON sync_metadata
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Step 2.2: Rollback Migration

File: `migrations/20260216000000_initial_schema.down.sql`

```sql
DROP TRIGGER IF EXISTS update_sync_metadata_updated_at ON sync_metadata;
DROP TRIGGER IF EXISTS update_shipments_updated_at ON shipments;
DROP TRIGGER IF EXISTS update_vehicles_updated_at ON vehicles;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_tenants_updated_at ON tenants;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS sync_metadata;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS proof_of_deliveries;
DROP TABLE IF EXISTS tracking_events;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS vehicles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;

DROP EXTENSION IF EXISTS postgis;
```

---

## Phase 3: sqlc Configuration & Queries

### Step 3.1: sqlc.yaml

File: `sqlc/sqlc.yaml`

```yaml
version: "2"
sql:
  - schema: "../migrations"
    queries: "./queries"
    engine: "postgresql"
    gen:
      go:
        package: "db"
        out: "../internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_db_tags: true
        emit_interface: true
        emit_exact_table_names: false
        emit_empty_slices: true
        emit_all_enum_values: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "pg_catalog.timestamp with time zone"
            go_type: "time.Time"
          - db_type: "pg_catalog.timestamp without time zone"
            go_type: "time.Time"
          - column: "tracking_events.coordinates"
            go_type: "github.com/jackc/pgx/v5/pgtype.Point"
          - column: "shipments.origin_coordinates"
            go_type: "github.com/jackc/pgx/v5/pgtype.Point"
          - column: "shipments.destination_coordinates"
            go_type: "github.com/jackc/pgx/v5/pgtype.Point"
          - column: "proof_of_deliveries.delivery_coordinates"
            go_type: "github.com/jackc/pgx/v5/pgtype.Point"
```

### Step 3.2: User Queries

File: `sqlc/queries/users.sql`

```sql
-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND tenant_id = $2;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByPhoneAndTenant :one
SELECT * FROM users
WHERE phone = $1 AND tenant_id = $2;

-- name: GetUserByEmailAndTenant :one
SELECT * FROM users
WHERE email = $1 AND tenant_id = $2;

-- name: ListUsersByTenant :many
SELECT * FROM users
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListDriversByTenant :many
SELECT * FROM users
WHERE tenant_id = $1 AND role = 'driver'
ORDER BY full_name
LIMIT $2 OFFSET $3;

-- name: CreateUser :one
INSERT INTO users (
    tenant_id, role, full_name, phone, email, password_hash, pin_hash, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET 
    full_name = COALESCE(sqlc.narg(full_name), full_name),
    phone = COALESCE(sqlc.narg(phone), phone),
    email = COALESCE(sqlc.narg(email), email),
    password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
    pin_hash = COALESCE(sqlc.narg(pin_hash), pin_hash),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteUser :exec
UPDATE users
SET is_active = false
WHERE id = $1 AND tenant_id = $2;

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND is_active = true;

-- name: CountDriversByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND role = 'driver' AND is_active = true;
```

### Step 3.3: Tenant Queries

File: `sqlc/queries/tenants.sql`

```sql
-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: CreateTenant :one
INSERT INTO tenants (name, slug, plan, max_drivers)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateTenant :one
UPDATE tenants
SET
    name = COALESCE(sqlc.narg(name), name),
    plan = COALESCE(sqlc.narg(plan), plan),
    max_drivers = COALESCE(sqlc.narg(max_drivers), max_drivers),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1
RETURNING *;
```

### Step 3.4: Session Queries

File: `sqlc/queries/sessions.sql`

```sql
-- name: CreateSession :one
INSERT INTO sessions (
    user_id, tenant_id, refresh_token_hash, device_id, user_agent, ip_address, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE refresh_token_hash = $1;

-- name: GetActiveSessionByTokenHash :one
SELECT * FROM sessions 
WHERE refresh_token_hash = $1 
  AND revoked_at IS NULL 
  AND expires_at > NOW();

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();
```

### Step 3.5: Shipment Queries

File: `sqlc/queries/shipments.sql`

```sql
-- name: GetShipmentByID :one
SELECT * FROM shipments
WHERE id = $1 AND tenant_id = $2;

-- name: GetShipmentByTrackingNumber :one
SELECT * FROM shipments
WHERE tracking_number = $1;

-- name: ListShipmentsByTenant :many
SELECT * FROM shipments
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1 AND tenant_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListShipmentsByStatus :many
SELECT * FROM shipments
WHERE tenant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListActiveShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1 
  AND tenant_id = $2
  AND status IN ('assigned', 'in_transit', 'delayed', 'arrived')
ORDER BY created_at DESC;

-- name: CreateShipment :one
INSERT INTO shipments (
    tenant_id, tracking_number,
    origin_address, origin_coordinates,
    destination_address, destination_coordinates,
    customer_name, customer_phone,
    cargo_description, cargo_weight, cargo_value, special_instructions,
    driver_id, vehicle_id,
    status, status_note,
    estimated_delivery,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
) RETURNING *;

-- name: UpdateShipment :one
UPDATE shipments
SET
    destination_address = COALESCE(sqlc.narg(destination_address), destination_address),
    destination_coordinates = COALESCE(sqlc.narg(destination_coordinates), destination_coordinates),
    customer_name = COALESCE(sqlc.narg(customer_name), customer_name),
    customer_phone = COALESCE(sqlc.narg(customer_phone), customer_phone),
    cargo_description = COALESCE(sqlc.narg(cargo_description), cargo_description),
    cargo_weight = COALESCE(sqlc.narg(cargo_weight), cargo_weight),
    special_instructions = COALESCE(sqlc.narg(special_instructions), special_instructions),
    driver_id = COALESCE(sqlc.narg(driver_id), driver_id),
    vehicle_id = COALESCE(sqlc.narg(vehicle_id), vehicle_id),
    status = COALESCE(sqlc.narg(status), status),
    status_note = COALESCE(sqlc.narg(status_note), status_note),
    status_reason = COALESCE(sqlc.narg(status_reason), status_reason),
    actual_delivery = COALESCE(sqlc.narg(actual_delivery), actual_delivery)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: AssignDriverToShipment :one
UPDATE shipments
SET driver_id = $1, status = 'assigned', updated_at = NOW()
WHERE id = $2 AND tenant_id = $3
RETURNING *;

-- name: UpdateShipmentStatus :one
UPDATE shipments
SET status = $1, status_note = $2, status_reason = $3, updated_at = NOW()
WHERE id = $4 AND tenant_id = $5
RETURNING *;

-- name: CountShipmentsByTenant :one
SELECT COUNT(*) FROM shipments WHERE tenant_id = $1;

-- name: SearchShipments :many
SELECT * FROM shipments
WHERE tenant_id = $1
  AND (
    tracking_number ILIKE '%' || $2 || '%'
    OR customer_name ILIKE '%' || $2 || '%'
    OR customer_phone ILIKE '%' || $2 || '%'
  )
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetShipmentsForSync :many
SELECT * FROM shipments
WHERE driver_id = $1
  AND tenant_id = $2
  AND (status IN ('assigned', 'in_transit', 'delayed', 'arrived') OR updated_at > $3)
ORDER BY created_at DESC;
```

### Step 3.6: Tracking Event Queries

File: `sqlc/queries/tracking.sql`

```sql
-- name: CreateTrackingEvent :one
INSERT INTO tracking_events (
    tenant_id, shipment_id, driver_id,
    coordinates, accuracy_meters, speed_kph, heading,
    event_type, status, note,
    recorded_at, device_id, battery_level
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: CreateTrackingEventBatch :many
INSERT INTO tracking_events (
    tenant_id, shipment_id, driver_id,
    coordinates, accuracy_meters, speed_kph, heading,
    event_type, status, note,
    recorded_at, device_id, battery_level
) VALUES
(SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::uuid[], $4::geometry[], $5::decimal[], $6::decimal[], $7::decimal[], $8::text[], $9::text[], $10::text[], $11::timestamptz[], $12::text[], $13::int[]))
RETURNING *;

-- name: ListTrackingEventsByShipment :many
SELECT * FROM tracking_events
WHERE shipment_id = $1 AND tenant_id = $2
ORDER BY recorded_at DESC
LIMIT $3 OFFSET $4;

-- name: ListTrackingEventsByDriver :many
SELECT * FROM tracking_events
WHERE driver_id = $1 AND tenant_id = $2
ORDER BY recorded_at DESC
LIMIT $3 OFFSET $4;

-- name: GetTrackingEventsForSync :many
SELECT * FROM tracking_events
WHERE driver_id = $1
  AND tenant_id = $2
  AND recorded_at > $3
  AND synced_at IS NULL
ORDER BY recorded_at ASC;

-- name: MarkTrackingEventsSynced :exec
UPDATE tracking_events
SET synced_at = NOW()
WHERE id = ANY($1::uuid[]);

-- name: CountTrackingEventsByShipment :one
SELECT COUNT(*) FROM tracking_events WHERE shipment_id = $1;
```

### Step 3.7: POD Queries

File: `sqlc/queries/pod.sql`

```sql
-- name: CreatePOD :one
INSERT INTO proof_of_deliveries (
    tenant_id, shipment_id, driver_id,
    recipient_name, recipient_phone,
    signature_data, signature_url, photo_urls,
    delivery_address, delivery_coordinates,
    delivery_notes, location_verified, location_mismatch_meters,
    recorded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
) RETURNING *;

-- name: GetPODByShipmentID :one
SELECT * FROM proof_of_deliveries
WHERE shipment_id = $1 AND tenant_id = $2;

-- name: UpdatePODSyncedAt :exec
UPDATE proof_of_deliveries
SET synced_at = NOW()
WHERE id = $1;

-- name: GetUnsyncedPODs :many
SELECT * FROM proof_of_deliveries
WHERE driver_id = $1 AND tenant_id = $2 AND synced_at IS NULL
ORDER BY recorded_at ASC;
```

---

## Phase 4: Configuration & Infrastructure

### Step 4.1: Configuration

File: `internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port string
	
	// Database
	DatabaseURL string
	
	// JWT
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	JWTIssuer         string
	
	// Logging
	LogLevel string
	
	// Environment
	Environment string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable"),
		JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", "keys/private.pem"),
		JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", "keys/public.pem"),
		JWTAccessTTL:      getDurationEnv("JWT_ACCESS_TTL", time.Hour),
		JWTRefreshTTL:     getDurationEnv("JWT_REFRESH_TTL", 30*24*time.Hour),
		JWTIssuer:         getEnv("JWT_ISSUER", "logistics.et"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		Environment:       getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
```

### Step 4.2: Database Connection

File: `internal/db/db.go`

```go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
```

### Step 4.3: Response Utilities

File: `pkg/response/response.go`

```go
package response

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Error  *Error      `json:"error,omitempty"`
	Meta   *Meta       `json:"meta,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Meta struct {
	RequestID string `json:"request_id,omitempty"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	Total     int    `json:"total,omitempty"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	resp := Response{Data: data}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		resp.Meta = &Meta{RequestID: requestID}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func ErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...any) {
	resp := Response{
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	if len(details) > 0 {
		resp.Error.Details = details[0]
	}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		resp.Meta = &Meta{RequestID: requestID}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func PaginatedJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}, page, perPage, total int) {
	resp := Response{
		Data: data,
		Meta: &Meta{
			RequestID: middleware.GetReqID(r.Context()),
			Page:      page,
			PerPage:   perPage,
			Total:     total,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
```

### Step 4.4: Validation

File: `pkg/validation/validation.go`

```go
package validation

import (
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func Init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
	
	// Register custom validations
	validate.RegisterValidation("ethiopian_phone", validateEthiopianPhone)
	validate.RegisterValidation("pin", validatePIN)
}

func Get() *validator.Validate {
	return validate
}

func validateEthiopianPhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	// Ethiopian phone: starts with 09 or 07 (10 digits) or +2519 or +2517 (13 digits)
	if len(phone) == 10 && (phone[:2] == "09" || phone[:2] == "07") {
		return true
	}
	if len(phone) == 13 && phone[:4] == "+251" && (phone[4:5] == "9" || phone[4:5] == "7") {
		return true
	}
	return false
}

func validatePIN(fl validator.FieldLevel) bool {
	pin := fl.Field().String()
	if len(pin) < 4 || len(pin) > 6 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type ValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Error string `json:"error"`
}

func FormatErrors(err error) []ValidationError {
	var errors []ValidationError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errors = append(errors, ValidationError{
				Field: e.Field(),
				Tag:   e.Tag(),
				Error: e.Error(),
			})
		}
	}
	return errors
}
```

### Step 4.5: Hashing

File: `pkg/hash/hash.go`

```go
package hash

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultCost = 12
	PINCost     = 10
)

func Password(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func PIN(pin string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pin), PINCost)
	return string(bytes), err
}

func CheckPIN(pin, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
	return err == nil
}
```

---

## Phase 5: JWT Package

### Step 5.1: Key Management

File: `pkg/jwt/keys.go`

```go
package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS1 first
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

func GenerateKeyPair(privatePath, publicPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Write private key
	privateFile, err := os.Create(privatePath)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateFile.Close()

	privatePEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	if err := pem.Encode(privateFile, privatePEM); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	publicFile, err := os.Create(publicPath)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicFile.Close()

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	if err := pem.Encode(publicFile, publicPEM); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}
```

### Step 5.2: JWT Utilities

File: `pkg/jwt/jwt.go`

```go
package jwt

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
}

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, issuer string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *JWTManager) GenerateAccessToken(userID, tenantID, role, phone, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:     m.issuer,
			Subject:    userID,
			ExpiresAt:  jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:   jwt.NewNumericDate(now),
			NotBefore:  jwt.NewNumericDate(now),
		},
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		Phone:    phone,
		Email:    email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func (m *JWTManager) GenerateRefreshToken(userID, tenantID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:     m.issuer,
		Subject:    userID,
		ExpiresAt:  jwt.NewNumericDate(now.Add(m.refreshTTL)),
		IssuedAt:   jwt.NewNumericDate(now),
		NotBefore:  jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func (m *JWTManager) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
```

---

## Phase 6: Middleware

### Step 6.1: Authentication Middleware

File: `internal/middleware/auth.go`

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourusername/logistics/pkg/jwt"
	"github.com/yourusername/logistics/pkg/response"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	TenantIDKey contextKey = "tenantID"
	RoleKey     contextKey = "role"
	ClaimsKey   contextKey = "claims"
)

func Auth(jwtManager *jwt.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format")
				return
			}

			claims, err := jwtManager.Validate(parts[1])
			if err != nil {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, TenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			ctx = context.WithValue(ctx, ClaimsKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

func GetTenantID(ctx context.Context) string {
	if id, ok := ctx.Value(TenantIDKey).(string); ok {
		return id
	}
	return ""
}

func GetRole(ctx context.Context) string {
	if role, ok := ctx.Value(RoleKey).(string); ok {
		return role
	}
	return ""
}

func GetClaims(ctx context.Context) *jwt.Claims {
	if claims, ok := ctx.Value(ClaimsKey).(*jwt.Claims); ok {
		return claims
	}
	return nil
}
```

### Step 6.2: Role-Based Access Control

File: `internal/middleware/rbac.go`

```go
package middleware

import (
	"net/http"

	"github.com/yourusername/logistics/pkg/response"
)

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			if role == "" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "No role found in context")
				return
			}

			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.ErrorJSON(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		})
	}
}

func RequireDriver() func(http.Handler) http.Handler {
	return RequireRole("driver")
}

func RequireDispatcher() func(http.Handler) http.Handler {
	return RequireRole("dispatcher", "fleet_manager", "admin")
}

func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole("admin", "platform_admin")
}
```

### Step 6.3: Logging Middleware

File: `internal/middleware/logging.go`

```go
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

func Logger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info().
					Str("request_id", middleware.GetReqID(r.Context())).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("query", r.URL.RawQuery).
					Int("status", ww.Status()).
					Int("bytes", ww.BytesWritten()).
					Dur("duration", time.Since(start)).
					Str("remote_addr", r.RemoteAddr).
					Str("tenant_id", GetTenantID(r.Context())).
					Str("user_id", GetUserID(r.Context())).
					Msg("request completed")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
```

---

## Phase 7: Service Layer

### Step 7.1: Auth Service

File: `internal/service/auth.go`

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/yourusername/logistics/internal/db"
	"github.com/yourusername/logistics/internal/model"
	"github.com/yourusername/logistics/pkg/hash"
	"github.com/yourusername/logistics/pkg/jwt"
)

type AuthService struct {
	queries    db.Querier
	jwtManager *jwt.JWTManager
}

func NewAuthService(queries db.Querier, jwtManager *jwt.JWTManager) *AuthService {
	return &AuthService{
		queries:    queries,
		jwtManager: jwtManager,
	}
}

type DriverLoginInput struct {
	Phone string `json:"phone" validate:"required,ethiopian_phone"`
	PIN   string `json:"pin" validate:"required,pin"`
}

type DispatcherLoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         *model.UserResponse
}

func (s *AuthService) DriverLogin(ctx context.Context, input DriverLoginInput) (*LoginOutput, error) {
	user, err := s.queries.GetUserByPhone(ctx, input.Phone)
	if err != nil {
		return nil, errors.New("invalid phone or PIN")
	}

	if !user.IsActive {
		return nil, errors.New("account is inactive")
	}

	if user.Role != "driver" {
		return nil, errors.New("invalid login type for this endpoint")
	}

	if !hash.CheckPIN(input.PIN, user.PinHash.String) {
		return nil, errors.New("invalid phone or PIN")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) DispatcherLogin(ctx context.Context, input DispatcherLoginInput) (*LoginOutput, error) {
	user, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if !user.IsActive {
		return nil, errors.New("account is inactive")
	}

	if user.Role == "driver" {
		return nil, errors.New("invalid login type for this endpoint")
	}

	if !hash.CheckPassword(input.Password, user.PasswordHash.String) {
		return nil, errors.New("invalid email or password")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginOutput, error) {
	claims, err := s.jwtManager.Validate(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	user, err := s.queries.GetUserByID(ctx, claims.Subject, claims.TenantID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if !user.IsActive {
		return nil, errors.New("account is inactive")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := sha256.Sum256([]byte(refreshToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	session, err := s.queries.GetActiveSessionByTokenHash(ctx, tokenHashStr)
	if err != nil {
		return nil
	}

	return s.queries.RevokeSession(ctx, session.ID)
}

func (s *AuthService) generateTokens(ctx context.Context, user *db.User) (*LoginOutput, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(
		user.ID.String(),
		user.TenantID.String(),
		user.Role,
		user.Phone.String,
		user.Email.String,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(
		user.ID.String(),
		user.TenantID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenHash := sha256.Sum256([]byte(refreshToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	_, err = s.queries.CreateSession(ctx, db.CreateSessionParams{
		UserID:           user.ID,
		TenantID:         user.TenantID,
		RefreshTokenHash: tokenHashStr,
		ExpiresAt:        time.Now().Add(s.jwtManager.RefreshTTL()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         model.UserFromDB(user).ToResponse(),
	}, nil
}
```

### Step 7.2: Shipment Service

File: `internal/service/shipment.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/yourusername/logistics/internal/db"
	"github.com/yourusername/logistics/internal/model"
)

type ShipmentService struct {
	queries db.Querier
}

func NewShipmentService(queries db.Querier) *ShipmentService {
	return &ShipmentService{queries: queries}
}

type CreateShipmentInput struct {
	OriginAddress      string  `json:"origin_address" validate:"required"`
	OriginLat          float64 `json:"origin_lat"`
	OriginLng          float64 `json:"origin_lng"`
	DestinationAddress string  `json:"destination_address" validate:"required"`
	DestinationLat     float64 `json:"destination_lat" validate:"required"`
	DestinationLng     float64 `json:"destination_lng" validate:"required"`
	CustomerName       string  `json:"customer_name" validate:"required"`
	CustomerPhone      string  `json:"customer_phone" validate:"required,ethiopian_phone"`
	CargoDescription   string  `json:"cargo_description"`
	CargoWeight        float64 `json:"cargo_weight"`
	CargoValue         float64 `json:"cargo_value"`
	SpecialInstructions string `json:"special_instructions"`
	DriverID           string  `json:"driver_id"`
	VehicleID          string  `json:"vehicle_id"`
}

type UpdateShipmentInput struct {
	DestinationAddress string  `json:"destination_address"`
	DestinationLat     float64 `json:"destination_lat"`
	DestinationLng     float64 `json:"destination_lng"`
	CustomerName       string  `json:"customer_name"`
	CustomerPhone      string  `json:"customer_phone"`
	CargoDescription   string  `json:"cargo_description"`
	SpecialInstructions string `json:"special_instructions"`
}

type UpdateStatusInput struct {
	Status       string `json:"status" validate:"required,oneof=pending assigned in_transit delayed arrived delivered issue cancelled"`
	StatusNote   string `json:"status_note"`
	StatusReason string `json:"status_reason"`
}

type AssignDriverInput struct {
	DriverID string `json:"driver_id" validate:"required,uuid"`
}

func (s *ShipmentService) Create(ctx context.Context, tenantID, createdBy string, input CreateShipmentInput) (*model.Shipment, error) {
	trackingNumber := generateTrackingNumber()

	driverID := pgtype.UUID{}
	if input.DriverID != "" {
		if err := driverID.Scan(input.DriverID); err != nil {
			return nil, fmt.Errorf("invalid driver ID: %w", err)
		}
	}

	vehicleID := pgtype.UUID{}
	if input.VehicleID != "" {
		if err := vehicleID.Scan(input.VehicleID); err != nil {
			return nil, fmt.Errorf("invalid vehicle ID: %w", err)
		}
	}

	createdByUUID := pgtype.UUID{}
	if err := createdByUUID.Scan(createdBy); err != nil {
		return nil, fmt.Errorf("invalid created_by ID: %w", err)
	}

	params := db.CreateShipmentParams{
		TenantID:           pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true},
		TrackingNumber:     trackingNumber,
		OriginAddress:      input.OriginAddress,
		DestinationAddress: input.DestinationAddress,
		CustomerName:       input.CustomerName,
		CustomerPhone:      input.CustomerPhone,
		CargoDescription:   pgtype.Text{String: input.CargoDescription, Valid: input.CargoDescription != ""},
		CargoWeight:        pgtype.Numeric{Float64: input.CargoWeight, Valid: input.CargoWeight > 0},
		CargoValue:         pgtype.Numeric{Float64: input.CargoValue, Valid: input.CargoValue > 0},
		SpecialInstructions: pgtype.Text{String: input.SpecialInstructions, Valid: input.SpecialInstructions != ""},
		DriverID:           driverID,
		VehicleID:          vehicleID,
		Status:             "pending",
		CreatedBy:          createdByUUID,
	}

	shipment, err := s.queries.CreateShipment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	return model.ShipmentFromDB(&shipment), nil
}

func (s *ShipmentService) GetByID(ctx context.Context, tenantID, shipmentID string) (*model.Shipment, error) {
	id := pgtype.UUID{Bytes: uuid.MustParse(shipmentID), Valid: true}
	tid := pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true}

	shipment, err := s.queries.GetShipmentByID(ctx, id, tid)
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	return model.ShipmentFromDB(&shipment), nil
}

func (s *ShipmentService) GetByTrackingNumber(ctx context.Context, trackingNumber string) (*model.Shipment, error) {
	shipment, err := s.queries.GetShipmentByTrackingNumber(ctx, trackingNumber)
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	return model.ShipmentFromDB(&shipment), nil
}

func (s *ShipmentService) ListByTenant(ctx context.Context, tenantID string, page, perPage int) ([]model.Shipment, int, error) {
	tid := pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true}
	offset := (page - 1) * perPage

	shipments, err := s.queries.ListShipmentsByTenant(ctx, db.ListShipmentsByTenantParams{
		TenantID: tid,
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list shipments: %w", err)
	}

	total, err := s.queries.CountShipmentsByTenant(ctx, tid)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count shipments: %w", err)
	}

	result := make([]model.Shipment, len(shipments))
	for i, s := range shipments {
		result[i] = *model.ShipmentFromDB(&s)
	}

	return result, int(total), nil
}

func (s *ShipmentService) AssignDriver(ctx context.Context, tenantID, shipmentID, driverID string) (*model.Shipment, error) {
	did := pgtype.UUID{Bytes: uuid.MustParse(driverID), Valid: true}
	sid := pgtype.UUID{Bytes: uuid.MustParse(shipmentID), Valid: true}
	tid := pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true}

	shipment, err := s.queries.AssignDriverToShipment(ctx, db.AssignDriverToShipmentParams{
		DriverID: did,
		ID:       sid,
		TenantID: tid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assign driver: %w", err)
	}

	return model.ShipmentFromDB(&shipment), nil
}

func (s *ShipmentService) UpdateStatus(ctx context.Context, tenantID, shipmentID string, input UpdateStatusInput) (*model.Shipment, error) {
	sid := pgtype.UUID{Bytes: uuid.MustParse(shipmentID), Valid: true}
	tid := pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true}

	shipment, err := s.queries.UpdateShipmentStatus(ctx, db.UpdateShipmentStatusParams{
		Status:       input.Status,
		StatusNote:   pgtype.Text{String: input.StatusNote, Valid: input.StatusNote != ""},
		StatusReason: pgtype.Text{String: input.StatusReason, Valid: input.StatusReason != ""},
		ID:           sid,
		TenantID:     tid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	return model.ShipmentFromDB(&shipment), nil
}

func generateTrackingNumber() string {
	return fmt.Sprintf("ET-%s-%04d", time.Now().Format("20060102"), time.Now().Nanosecond()/100000)
}
```

### Step 7.3: Sync Service

File: `internal/service/sync.go`

```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/yourusername/logistics/internal/db"
	"github.com/yourusername/logistics/internal/model"
)

type SyncService struct {
	queries db.Querier
}

func NewSyncService(queries db.Querier) *SyncService {
	return &SyncService{queries: queries}
}

type SyncRequest struct {
	DeviceID    string           `json:"device_id"`
	LastSyncAt  time.Time        `json:"last_sync_at"`
	Events      []TrackingEvent  `json:"events"`
	PODs        []PODInput       `json:"pods"`
	Statuses    []StatusUpdate  `json:"statuses"`
	Battery     int              `json:"battery_level"`
	StorageUsed int              `json:"storage_remaining_kb"`
}

type TrackingEvent struct {
	ShipmentID   string  `json:"shipment_id"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	Accuracy     float64 `json:"accuracy"`
	Speed        float64 `json:"speed"`
	Heading      float64 `json:"heading"`
	EventType    string  `json:"event_type"`
	Status       string  `json:"status,omitempty"`
	Note         string  `json:"note,omitempty"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type PODInput struct {
	ShipmentID      string  `json:"shipment_id"`
	RecipientName   string  `json:"recipient_name"`
	RecipientPhone  string  `json:"recipient_phone"`
	SignatureData   string  `json:"signature_data"`
	PhotoURLs       []string `json:"photo_urls"`
	DeliveryAddress string  `json:"delivery_address"`
	DeliveryLat     float64 `json:"delivery_lat"`
	DeliveryLng     float64 `json:"delivery_lng"`
	DeliveryNotes   string  `json:"delivery_notes"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type StatusUpdate struct {
	ShipmentID string    `json:"shipment_id"`
	Status     string    `json:"status"`
	Note       string    `json:"note"`
	Reason     string    `json:"reason"`
	RecordedAt time.Time `json:"recorded_at"`
}

type SyncResponse struct {
	ServerTime    time.Time         `json:"server_time"`
	EventsReceived int              `json:"events_received"`
	Conflicts     []Conflict        `json:"conflicts,omitempty"`
	Pull          SyncPullData      `json:"pull"`
}

type Conflict struct {
	Type        string `json:"type"`
	ShipmentID  string `json:"shipment_id"`
	LocalValue  string `json:"local_value"`
	ServerValue string `json:"server_value"`
	Resolution  string `json:"resolution"`
}

type SyncPullData struct {
	Shipments []model.Shipment `json:"shipments"`
	Messages  []string         `json:"messages"`
	Updates   []string         `json:"updates"`
}

func (s *SyncService) Sync(ctx context.Context, tenantID, driverID string, req SyncRequest) (*SyncResponse, error) {
	tid := pgtype.UUID{Bytes: uuid.MustParse(tenantID), Valid: true}
	did := pgtype.UUID{Bytes: uuid.MustParse(driverID), Valid: true}

	now := time.Now()
	eventsReceived := 0
	var conflicts []Conflict

	// Process tracking events
	for _, event := range req.Events {
		sid := pgtype.UUID{Bytes: uuid.MustParse(event.ShipmentID), Valid: true}
		
		_, err := s.queries.CreateTrackingEvent(ctx, db.CreateTrackingEventParams{
			TenantID:      tid,
			ShipmentID:    sid,
			DriverID:      did,
			Coordinates:   pgtype.Point{P: pgtype.Vec2{X: event.Lng, Y: event.Lat}, Valid: true},
			AccuracyMeters: pgtype.Numeric{Float64: event.Accuracy, Valid: event.Accuracy > 0},
			SpeedKph:      pgtype.Numeric{Float64: event.Speed, Valid: event.Speed > 0},
			Heading:       pgtype.Numeric{Float64: event.Heading, Valid: event.Heading > 0},
			EventType:     event.EventType,
			Status:        pgtype.Text{String: event.Status, Valid: event.Status != ""},
			Note:          pgtype.Text{String: event.Note, Valid: event.Note != ""},
			RecordedAt:    event.RecordedAt,
			DeviceID:      pgtype.Text{String: req.DeviceID, Valid: req.DeviceID != ""},
			BatteryLevel:  pgtype.Int4{Int32: int32(req.Battery), Valid: req.Battery > 0},
		})
		if err == nil {
			eventsReceived++
		}
	}

	// Process PODs
	for _, pod := range req.PODs {
		sid := pgtype.UUID{Bytes: uuid.MustParse(pod.ShipmentID), Valid: true}
		
		_, err := s.queries.CreatePOD(ctx, db.CreatePODParams{
			TenantID:           tid,
			ShipmentID:         sid,
			DriverID:           did,
			RecipientName:      pod.RecipientName,
			RecipientPhone:     pgtype.Text{String: pod.RecipientPhone, Valid: pod.RecipientPhone != ""},
			SignatureData:      pgtype.Text{String: pod.SignatureData, Valid: pod.SignatureData != ""},
			PhotoUrls:          pod.PhotoURLs,
			DeliveryAddress:    pgtype.Text{String: pod.DeliveryAddress, Valid: pod.DeliveryAddress != ""},
			DeliveryCoordinates: pgtype.Point{P: pgtype.Vec2{X: pod.DeliveryLng, Y: pod.DeliveryLat}, Valid: pod.DeliveryLat != 0},
			DeliveryNotes:      pgtype.Text{String: pod.DeliveryNotes, Valid: pod.DeliveryNotes != ""},
			RecordedAt:         pod.RecordedAt,
		})
		if err == nil {
			eventsReceived++
		}
	}

	// Process status updates
	for _, status := range req.Statuses {
		_, err := s.UpdateShipmentStatus(ctx, tenantID, status.ShipmentID, UpdateStatusInput{
			Status:       status.Status,
			StatusNote:   status.Note,
			StatusReason: status.Reason,
		})
		if err == nil {
			eventsReceived++
		}
	}

	// Pull data for driver
	shipments, _ := s.queries.GetShipmentsForSync(ctx, db.GetShipmentsForSyncParams{
		DriverID: did,
		TenantID: tid,
		Column4:   req.LastSyncAt,
	})

	pullData := SyncPullData{}
	for _, s := range shipments {
		pullData.Shipments = append(pullData.Shipments, *model.ShipmentFromDB(&s))
	}

	return &SyncResponse{
		ServerTime:     now,
		EventsReceived: eventsReceived,
		Conflicts:      conflicts,
		Pull:           pullData,
	}, nil
}
```

---

## Phase 8: Handlers

### Step 8.1: Handler Dependencies

File: `internal/handler/handler.go`

```go
package handler

import (
	"github.com/yourusername/logistics/internal/service"
)

type Handler struct {
	Auth     *service.AuthService
	Shipment *service.ShipmentService
	Sync     *service.SyncService
	User     *service.UserService
	Tracking *service.TrackingService
	POD      *service.PODService
}

func New(
	auth *service.AuthService,
	shipment *service.ShipmentService,
	sync *service.SyncService,
	user *service.UserService,
	tracking *service.TrackingService,
	pod *service.PODService,
) *Handler {
	return &Handler{
		Auth:     auth,
		Shipment: shipment,
		Sync:     sync,
		User:     user,
		Tracking: tracking,
		POD:      pod,
	}
}
```

### Step 8.2: Auth Handlers

File: `internal/handler/auth.go`

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/yourusername/logistics/internal/service"
	"github.com/yourusername/logistics/pkg/response"
	"github.com/yourusername/logistics/pkg/validation"
)

func (h *Handler) DriverLogin(w http.ResponseWriter, r *http.Request) {
	var input service.DriverLoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	result, err := h.Auth.DriverLogin(r.Context(), input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) DispatcherLogin(w http.ResponseWriter, r *http.Request) {
	var input service.DispatcherLoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	result, err := h.Auth.DispatcherLogin(r.Context(), input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	result, err := h.Auth.RefreshToken(r.Context(), input.RefreshToken)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.Auth.Logout(r.Context(), input.RefreshToken); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to logout")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}
```

### Step 8.3: Shipment Handlers

File: `internal/handler/shipment.go`

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/logistics/internal/middleware"
	"github.com/yourusername/logistics/internal/service"
	"github.com/yourusername/logistics/pkg/response"
	"github.com/yourusername/logistics/pkg/validation"
)

func (h *Handler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	var input service.CreateShipmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	shipment, err := h.Shipment.Create(r.Context(), tenantID, userID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, shipment)
}

func (h *Handler) GetShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	shipment, err := h.Shipment.GetByID(r.Context(), tenantID, shipmentID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Shipment not found")
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

func (h *Handler) ListShipments(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	shipments, total, err := h.Shipment.ListByTenant(r.Context(), tenantID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, shipments, page, perPage, total)
}

func (h *Handler) AssignDriver(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	var input service.AssignDriverInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	shipment, err := h.Shipment.AssignDriver(r.Context(), tenantID, shipmentID, input.DriverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

func (h *Handler) UpdateShipmentStatus(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	var input service.UpdateStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	shipment, err := h.Shipment.UpdateStatus(r.Context(), tenantID, shipmentID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}
```

### Step 8.4: Sync Handler

File: `internal/handler/sync.go`

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/yourusername/logistics/internal/middleware"
	"github.com/yourusername/logistics/internal/service"
	"github.com/yourusername/logistics/pkg/response"
)

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	var req service.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	driverID := middleware.GetUserID(r.Context())

	result, err := h.Sync.Sync(r.Context(), tenantID, driverID, req)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}
```

---

## Phase 9: Main Application

### Step 9.1: Routes Setup

File: `internal/handler/routes.go`

```go
package handler

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/yourusername/logistics/internal/middleware"
	"github.com/rs/zerolog"
)

func (h *Handler) Routes(logger zerolog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Throttle(100))

	// CORS
	r.Use(middleware.CORS())

	// Health check
	r.Get("/health", h.Health)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/auth/login/driver", h.DriverLogin)
		r.Post("/auth/login/dispatcher", h.DispatcherLogin)
		r.Post("/auth/refresh", h.RefreshToken)
		r.Get("/track/{tracking_number}", h.TrackShipment)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(h.jwtManager))

		// Auth
		r.Post("/auth/logout", h.Logout)

		// Sync (driver only)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireDriver())
			r.Post("/sync", h.Sync)
			r.Post("/tracking-events", h.CreateTrackingEvent)
			r.Post("/proof-of-delivery", h.CreatePOD)
		})

		// Dispatcher routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireDispatcher())

			// Shipments
			r.Post("/shipments", h.CreateShipment)
			r.Get("/shipments", h.ListShipments)
			r.Get("/shipments/{id}", h.GetShipment)
			r.Put("/shipments/{id}", h.UpdateShipment)
			r.Put("/shipments/{id}/assign", h.AssignDriver)
			r.Put("/shipments/{id}/status", h.UpdateShipmentStatus)

			// Drivers
			r.Get("/drivers", h.ListDrivers)
			r.Post("/drivers", h.CreateDriver)

			// Users
			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)
		})
	})

	return r
}
```

### Step 9.2: Main Entry Point

File: `cmd/api/main.go`

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourusername/logistics/internal/config"
	"github.com/yourusername/logistics/internal/db"
	"github.com/yourusername/logistics/internal/handler"
	"github.com/yourusername/logistics/internal/middleware"
	"github.com/yourusername/logistics/internal/service"
	"github.com/yourusername/logistics/pkg/jwt"
	"github.com/yourusername/logistics/pkg/validation"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if cfg.Environment == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger.Info().Msg("Starting logistics API")

	// Initialize validation
	validation.Init()

	// Connect to database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()
	logger.Info().Msg("Connected to database")

	// Load JWT keys
	privateKey, err := jwt.LoadPrivateKey(cfg.JWTPrivateKeyPath)
	if err != nil {
		logger.Warn().Msg("Generating new JWT key pair")
		if err := jwt.GenerateKeyPair(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath); err != nil {
			logger.Fatal().Err(err).Msg("Failed to generate JWT keys")
		}
		privateKey, err = jwt.LoadPrivateKey(cfg.JWTPrivateKeyPath)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to load private key after generation")
		}
	}

	publicKey, err := jwt.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load public key")
	}

	jwtManager := jwt.NewManager(privateKey, publicKey, cfg.JWTIssuer, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	// Initialize services
	queries := db.New(pool)
	authService := service.NewAuthService(queries, jwtManager)
	shipmentService := service.NewShipmentService(queries)
	syncService := service.NewSyncService(queries)
	userService := service.NewUserService(queries)
	trackingService := service.NewTrackingService(queries)
	podService := service.NewPODService(queries)

	// Initialize handlers
	h := handler.New(authService, shipmentService, syncService, userService, trackingService, podService, jwtManager)

	// Create router
	router := h.Routes(logger)

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server error")
		}
	}()

	logger.Info().Str("port", cfg.Port).Msg("Server started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	logger.Info().Msg("Server stopped")
}
```

---

## Phase 10: Testing

### Step 10.1: Integration Test Setup

File: `tests/integration/main_test.go`

```go
//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) {
	// Run tests
	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to database: %s", err)
	}

	// Run migrations
	// ... migration logic here

	cleanup := func() {
		pool.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}

	return pool, cleanup
}
```

### Step 10.2: Auth Integration Test

File: `tests/integration/auth_test.go`

```go
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/logistics/internal/handler"
	"github.com/yourusername/logistics/internal/service"
)

func TestDriverLogin(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup handler
	h := setupHandler(pool)

	tests := []struct {
		name       string
		input      service.DriverLoginInput
		wantStatus int
	}{
		{
			name: "valid login",
			input: service.DriverLoginInput{
				Phone: "0912345678",
				PIN:   "1234",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid phone",
			input: service.DriverLoginInput{
				Phone: "invalid",
				PIN:   "1234",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "wrong PIN",
			input: service.DriverLoginInput{
				Phone: "0912345678",
				PIN:   "0000",
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.input)
			req := httptest.NewRequest(http.MethodPost, "/auth/login/driver", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			h.DriverLogin(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}
```

---

## Phase 11: Development Workflow

### Step 11.1: Daily Development

```bash
# Start development environment
make docker-up

# Wait for postgres to be ready
sleep 5

# Run migrations
make migrate-up

# Generate sqlc code (after writing queries)
make sqlc

# Run the API
make run

# Run tests
make test

# Run integration tests
make test-integration
```

### Step 11.2: Adding a New Feature

Example: Adding vehicle management

1. Create migration:
```bash
make migrate-create
# Enter: add_vehicles_table
```

2. Write SQL query in `sqlc/queries/vehicles.sql`

3. Generate code:
```bash
make sqlc
```

4. Create domain model in `internal/model/vehicle.go`

5. Create service in `internal/service/vehicle.go`

6. Create handler in `internal/handler/vehicle.go`

7. Add routes in `internal/handler/routes.go`

8. Write tests in `tests/integration/vehicle_test.go`

9. Run tests:
```bash
make test-integration
```

---

## Phase 12: API Endpoints Summary

### Authentication

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | /auth/login/driver | Driver login (phone + PIN) | No |
| POST | /auth/login/dispatcher | Dispatcher login (email + password) | No |
| POST | /auth/refresh | Refresh access token | No |
| POST | /auth/logout | Invalidate refresh token | Yes |

### Shipments

| Method | Endpoint | Description | Auth | Role |
|--------|----------|-------------|------|------|
| POST | /shipments | Create shipment | Yes | Dispatcher+ |
| GET | /shipments | List shipments | Yes | Dispatcher+ |
| GET | /shipments/:id | Get shipment | Yes | All |
| PUT | /shipments/:id | Update shipment | Yes | Dispatcher+ |
| PUT | /shipments/:id/assign | Assign driver | Yes | Dispatcher+ |
| PUT | /shipments/:id/status | Update status | Yes | All |
| GET | /shipments/:id/tracking | Get tracking events | Yes | All |

### Sync (Driver App)

| Method | Endpoint | Description | Auth | Role |
|--------|----------|-------------|------|------|
| POST | /sync | Batch sync | Yes | Driver |
| POST | /tracking-events | Create tracking event | Yes | Driver |
| POST | /proof-of-delivery | Create POD | Yes | Driver |

### Users

| Method | Endpoint | Description | Auth | Role |
|--------|----------|-------------|------|------|
| GET | /users | List users | Yes | Dispatcher+ |
| POST | /users | Create user | Yes | Admin |
| GET | /drivers | List drivers | Yes | Dispatcher+ |
| POST | /drivers | Create driver | Yes | Admin |

### Public

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | /health | Health check | No |
| GET | /track/:tracking_number | Track shipment | No |

---

## Timeline Summary

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Phase 1: Project Foundation | Day 1-2 | Directory structure, dependencies, Makefile |
| Phase 2: Database Schema | Day 2-3 | Migrations, sqlc configuration |
| Phase 3: Infrastructure | Day 3-4 | Config, database connection, utilities |
| Phase 4: JWT Package | Day 4 | Key management, token generation |
| Phase 5: Middleware | Day 4-5 | Auth, RBAC, logging |
| Phase 6: Service Layer | Day 5-8 | Auth, shipment, sync services |
| Phase 7: Handlers | Day 8-10 | All HTTP handlers |
| Phase 8: Main Application | Day 10 | Routes, entry point |
| Phase 9: Testing | Day 11-12 | Integration tests |
| Phase 10: Documentation | Day 12-13 | README, API docs |
| Phase 11: Review & Polish | Day 13-14 | Code review, cleanup |

**Total: ~2-3 weeks for MVP backend**

---

*Document End*