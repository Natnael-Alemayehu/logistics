# Phase 2 Development Plan
## Ethiopian Logistics Tracking Platform - Backend

**Status**: Ready for Implementation
**Prerequisites**: Phase 1 completed (core MVP backend)
**Date**: February 2026

---

## Phase 1 Completion Summary

### ✅ Implemented

| Component | Status | Files |
|-----------|--------|-------|
| Project Structure | ✅ | Makefile, docker-compose.yml, Dockerfile |
| Database Schema | ✅ | migrations/20260216000000_initial_schema.up.sql |
| sqlc Generated Code | ✅ | internal/db/*.go |
| Configuration | ✅ | internal/config/config.go |
| JWT Package | ✅ | pkg/jwt/jwt.go, keys.go |
| Password/PIN Hashing | ✅ | pkg/hash/hash.go |
| Validation | ✅ | pkg/validation/validation.go |
| Response Utilities | ✅ | pkg/response/response.go |
| Auth Middleware | ✅ | internal/middleware/auth.go, rbac.go |
| CORS Middleware | ✅ | internal/middleware/cors.go |
| Logging Middleware | ✅ | internal/middleware/logging.go |
| Auth Service | ✅ | internal/service/auth.go |
| Shipment Service | ✅ | internal/service/shipment.go |
| Sync Service | ✅ | internal/service/sync.go |
| User Service | ✅ | internal/service/user.go |
| Auth Handlers | ✅ | internal/handler/auth.go |
| Shipment Handlers | ✅ | internal/handler/shipment.go |
| Sync Handler | ✅ | internal/handler/sync.go |
| User Handlers | ✅ | internal/handler/user.go |
| Routes | ✅ | internal/handler/routes.go |
| Main Entry Point | ✅ | cmd/api/main.go |

### ✅ Tested Endpoints

| Endpoint | Method | Status |
|----------|--------|--------|
| /health | GET | ✅ |
| /auth/login/driver | POST | ✅ |
| /auth/login/dispatcher | POST | ✅ |
| /auth/refresh | POST | ✅ |
| /auth/logout | POST | ✅ |
| /shipments | POST | ✅ |
| /shipments | GET | ✅ |
| /shipments/:id | GET | ✅ |
| /shipments/:id/assign | PUT | ✅ |
| /shipments/:id/status | PUT | ✅ |
| /my-shipments | GET | ✅ |
| /sync | POST | ✅ |
| /track/:tracking_number | GET | ✅ |
| /drivers | GET | ✅ |
| /drivers | POST | ✅ |
| /users | GET | ✅ |
| /users | POST | ✅ |

---

## Missing from MVP Requirements (FRD P1)

| FR | Requirement | Status |
|----|-------------|--------|
| FR-001.3 | SMS temporary credentials on user creation | ❌ |
| FR-001.4 | Force password change on first login | ❌ |
| FR-002.6 | Account lockout after 5 failed attempts | ❌ |
| FR-003.3 | View/revoke active sessions | ❌ |
| FR-009 | Full search/filter (by driver, origin, destination, date range) | Partial |
| FR-011 | Shipment editing | ❌ |
| FR-012 | Shipment cancellation with reason | ❌ |
| FR-018.3 | Delay status with reason codes | ❌ |
| FR-022 | Delivery location verification | ❌ |
| FR-024 | SMS fallback for status updates | ❌ |
| FR-032 | Vehicle management CRUD | ❌ |
| FR-033 | Real-time notifications & alerts | ❌ |
| FR-042 | Audit logging wired to all actions | ❌ |

---

## Part A: Complete MVP Requirements

### 1. Vehicle Management Service

**Priority**: P1 (FR-032)

**New Files:**
- `sqlc/queries/vehicles.sql`
- `internal/service/vehicle.go`
- `internal/handler/vehicle.go`

**Endpoints:**
| Method | Endpoint | Handler | Auth |
|--------|----------|---------|------|
| POST | /vehicles | CreateVehicle | Dispatcher+ |
| GET | /vehicles | ListVehicles | Dispatcher+ |
| GET | /vehicles/:id | GetVehicle | Dispatcher+ |
| PUT | /vehicles/:id | UpdateVehicle | Dispatcher+ |
| DELETE | /vehicles/:id | DeleteVehicle | Admin |

---

### 2. Shipment Edit & Cancel

**Priority**: P1 (FR-011, FR-012)

**Endpoints:**
| Method | Endpoint | Handler | Auth |
|--------|----------|---------|------|
| PUT | /shipments/:id | UpdateShipment | Dispatcher+ |
| POST | /shipments/:id/cancel | CancelShipment | Dispatcher+ |

---

### 3. Advanced Search & Filtering

**Priority**: P1 (FR-009)

**Endpoint:**
| Method | Endpoint | Handler |
|--------|----------|---------|
| GET | /shipments/search | SearchShipments |

**Filters:** status, driver_id, date_from, date_to, origin, destination, query

---

### 4. Delay Reason Codes

**Priority**: P1 (FR-018.3)

**Reason Codes:**
- road_conditions
- weather
- security_checkpoint
- mechanical_issue
- traffic
- other

---

### 5. Session Management

**Priority**: P1 (FR-003.3)

**Endpoints:**
| Method | Endpoint | Handler |
|--------|----------|---------|
| GET | /sessions | ListSessions |
| DELETE | /sessions/:id | RevokeSession |
| DELETE | /sessions/others | RevokeOtherSessions |

---

### 6. Rate Limiting

**Priority**: High (NFR-020)

**Limits:**
- 100 requests/minute per user
- 1000 requests/minute per API key

---

### 7. Account Lockout

**Priority**: P1 (FR-002.6)

**Rules:**
- 5 failed attempts → 15 minute lockout
- Log all attempts to audit_log

---

## Part B: Production Readiness

### 8. Audit Logging Service

**Priority**: P1 (FR-042)

**Logged Actions:**
- LOGIN, LOGOUT, LOGIN_FAILED
- SHIPMENT_CREATED, SHIPMENT_UPDATED, SHIPMENT_CANCELLED
- STATUS_CHANGED, DRIVER_ASSIGNED
- POD_CAPTURED
- USER_CREATED, USER_UPDATED

---

### 9. Integration Tests

**Priority**: High (NFR-029)

**Test Files:**
- tests/integration/main_test.go
- tests/integration/auth_test.go
- test/integration/shipment_test.go
- tests/integration/sync_test.go
- tests/integration/vehicle_test.go

---

### 10. OpenAPI Specification

**Priority**: Medium

**File:** `docs/api/openapi.yaml`

---

### 11. Enhanced Health Check

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "database": "connected",
  "timestamp": "2026-02-16T..."
}
```

---

### 12. Standardized Error Handling

**Error Codes:**
- VALIDATION_ERROR
- NOT_FOUND
- UNAUTHORIZED
- FORBIDDEN
- RATE_LIMITED
- INTERNAL_ERROR
- ACCOUNT_LOCKED

---

## Timeline

| Week | Tasks |
|------|-------|
| 1 | Vehicle service, Shipment edit/cancel |
| 2 | Advanced search, Delay reasons, Rate limiting |
| 3 | Account lockout, Session management |
| 4 | Audit logging, Integration tests |
| 5 | OpenAPI docs, Health check, Error handling |
| 6 | Polish, final testing |

---

## Files to Create/Modify Summary

### New Files (14)

```
migrations/20260217000000_add_lockout_columns.up.sql
migrations/20260217000000_add_lockout_columns.down.sql
sqlc/queries/vehicles.sql
sqlc/queries/audit.sql
internal/service/vehicle.go
internal/service/audit.go
internal/handler/vehicle.go
internal/middleware/ratelimit.go
tests/integration/main_test.go
tests/integration/auth_test.go
tests/integration/shipment_test.go
tests/integration/sync_test.go
tests/integration/vehicle_test.go
docs/api/openapi.yaml
```

### Modified Files (12)

```
sqlc/queries/shipments.sql
sqlc/queries/sessions.sql
sqlc/queries/users.sql
internal/service/shipment.go
internal/service/auth.go
internal/service/sync.go
internal/handler/shipment.go
internal/handler/auth.go
internal/handler/handler.go
internal/handler/routes.go
pkg/validation/validation.go
pkg/response/response.go
```

---

*Document End*