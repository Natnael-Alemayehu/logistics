# E2E Test Guide - Ethiopian Logistics Tracking Platform

This document provides comprehensive end-to-end testing instructions for the Ethiopian Logistics Tracking Platform, covering backend API, frontend web application, and mobile driver app.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Backend Verification](#backend-verification)
3. [Frontend Verification](#frontend-verification)
4. [Mobile App Verification](#mobile-app-verification)
5. [Service Startup](#service-startup)
6. [Test Scenarios](#test-scenarios)
7. [API Reference](#api-reference)

---

## Prerequisites

### Required Services

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL (PostGIS) | 5432 | Primary database |
| Redis | 6379 | Session cache, rate limiting |
| Backend API | 8080 | REST API server |
| Frontend | 3000 | Next.js web application |
| Mobile | 8081 | Expo development server |

### Environment Setup

#### 1. Start Infrastructure Services

```bash
cd /home/nate/Projects/logistics/backend

# Start PostgreSQL and Redis
make docker-up

# Wait for services to be healthy (5-10 seconds)
docker compose ps
```

#### 2. Database Migration

```bash
cd /home/nate/Projects/logistics/backend

# Run migrations
make migrate-up

# (Optional) Seed test data
make seed
```

#### 3. Generate JWT Keys (if not present)

```bash
cd /home/nate/Projects/logistics/backend
mkdir -p keys

# Generate private key
openssl genrsa -out keys/private.pem 2048

# Generate public key
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

---

## Backend Verification

### API Endpoints Summary

The backend provides the following API endpoints organized by category:

#### Authentication Endpoints

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/api/v1/auth/login/driver` | Driver login (phone + PIN) | No |
| POST | `/api/v1/auth/login/dispatcher` | Dispatcher/Admin login (email + password) | No |
| POST | `/api/v1/auth/refresh` | Refresh access token | No |
| POST | `/api/v1/auth/logout` | Logout user | Yes |
| POST | `/api/v1/auth/change-password` | Change password | Yes |
| POST | `/api/v1/auth/forgot-pin` | Reset driver PIN | Yes |

#### Driver-Specific Endpoints (Requires Driver Role)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/sync` | Sync offline data (events, PODs, statuses) |
| GET | `/api/v1/my-shipments` | Get assigned shipments |
| GET | `/api/v1/driver/stats` | Get driver delivery statistics |
| GET | `/api/v1/driver/vehicle` | Get assigned vehicle |

#### Shipment Endpoints (Requires Dispatcher Role)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/shipments` | Create shipment |
| GET | `/api/v1/shipments` | List shipments (paginated) |
| GET | `/api/v1/shipments/search` | Search shipments |
| GET | `/api/v1/shipments/{id}` | Get shipment details |
| PUT | `/api/v1/shipments/{id}` | Update shipment |
| PUT | `/api/v1/shipments/{id}/assign` | Assign driver |
| PUT | `/api/v1/shipments/{id}/status` | Update status |
| POST | `/api/v1/shipments/{id}/cancel` | Cancel shipment |
| GET | `/api/v1/shipments/{id}/tracking` | Get tracking events |
| GET | `/api/v1/shipments/{id}/pod` | Get proof of delivery |

#### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/track/{tracking_number}` | Track shipment (public) |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

### Verify Backend Compilation

```bash
cd /home/nate/Projects/logistics/backend

# Build the API
make build

# Run tests
make test
```

---

## Frontend Verification

### Available Pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | Landing | Public landing page |
| `/login` | Login | Authentication page |
| `/dashboard` | Dashboard | Main dispatcher dashboard |
| `/shipments` | Shipments | Shipment management |
| `/shipments/new` | New Shipment | Create shipment form |
| `/shipments/[id]` | Shipment Details | Single shipment view |
| `/drivers` | Drivers | Driver management |
| `/vehicles` | Vehicles | Vehicle management |
| `/reports` | Reports | Analytics and reports |
| `/settings` | Settings | Application settings |
| `/track` | Track | Public tracking page |
| `/admin` | Admin | Admin panel |
| `/admin/users` | Users | User management |
| `/admin/tenants` | Tenants | Tenant management |

### Verify Frontend Compilation

```bash
cd /home/nate/Projects/logistics/frontend

# Install dependencies
npm install

# Build the application
npm run build

# TypeScript check
npx tsc --noEmit
```

### API Client Configuration

The frontend uses an API client at `lib/api-client.ts`:
- Base URL: `process.env.NEXT_PUBLIC_API_URL` or `http://localhost:8080`
- Automatic token refresh on 401 responses
- Bearer token authentication

---

## Mobile App Verification

### Available Screens

| Screen | Path | Description |
|--------|------|-------------|
| Login | `(auth)/login.tsx` | Driver login (phone + PIN) |
| Forgot PIN | `(auth)/forgot-pin.tsx` | PIN recovery |
| Shipments List | `(main)/index.tsx` | Assigned shipments |
| Shipment Details | `(main)/shipment/[id].tsx` | Single shipment view |
| Map/Tracking | `(main)/map.tsx` | Live tracking map |
| Profile | `(main)/profile.tsx` | Driver profile |
| Settings | `(main)/settings.tsx` | App settings |
| POD Form | `pod/[shipmentId].tsx` | Proof of delivery |

### Verify Mobile Compilation

```bash
cd /home/nate/Projects/logistics/mobile

# Install dependencies
npm install

# TypeScript check
npx tsc --noEmit

# Start Expo development server
npx expo start
```

### API Integration

The mobile app uses a comprehensive API integration:

- **Auth Service** (`services/api/auth.ts`): Login, logout, token refresh
- **Shipments Service** (`services/api/shipments.ts`): Get shipments, update status
- **Sync Service** (`services/api/sync.ts`): Offline data synchronization
- **Driver Service** (`services/api/driver.ts`): Stats, vehicle assignment

---

## Service Startup

### Start Backend

```bash
cd /home/nate/Projects/logistics/backend

# Development mode with hot reload
make dev

# Or run directly
make run
```

The API will be available at `http://localhost:8080`

### Start Frontend

```bash
cd /home/nate/Projects/logistics/frontend

# Development mode
npm run dev

# Production build
npm run build && npm start
```

The frontend will be available at `http://localhost:3000`

### Start Mobile App

```bash
cd /home/nate/Projects/logistics/mobile

# Start Expo development server
npx expo start

# Run on specific platform
npx expo start --android
npx expo start --ios
```

The Expo dev server runs at `http://localhost:8081`

---

## Test Scenarios

### Scenario 1: Driver Login (Phone + PIN)

**Description**: Authenticate a driver using Ethiopian phone number and PIN.

#### Mobile App Flow

1. Open the mobile app
2. Enter phone number in format: `0911234567` or `+251912345678`
3. Enter 4-6 digit PIN
4. Tap "Sign In"
5. App performs initial sync after successful login

#### Expected API Calls

```
POST /api/v1/auth/login/driver
Content-Type: application/json

{
  "phone": "+251912345678",
  "pin": "1234"
}
```

**Success Response**:
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJSUzI1NiIs...",
    "user": {
      "id": "uuid",
      "phone": "+251912345678",
      "name": "Abebe Kebede"
    }
  }
}
```

**Error Responses**:
- `401 Unauthorized`: Invalid phone or PIN
- `403 Forbidden`: Account locked (too many failed attempts)

#### Post-Login: Initial Sync

```
POST /api/v1/sync
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "device_id": "device-uuid",
  "last_sync_at": "0001-01-01T00:00:00Z",
  "battery_level": 85,
  "storage_remaining_kb": 500000
}
```

---

### Scenario 2: View Assigned Shipments

**Description**: Driver views their list of assigned shipments.

#### Mobile App Flow

1. After login, user lands on shipments list screen
2. Tabs available: "Active", "Completed", "All"
3. Pull-to-refresh updates the list
4. Tap a shipment to view details

#### Expected API Calls

```
GET /api/v1/my-shipments
Authorization: Bearer {access_token}
```

**Success Response**:
```json
{
  "data": [
    {
      "id": "uuid",
      "tracking_number": "ET-2024-001234",
      "status": "in_transit",
      "origin": "Addis Ababa",
      "destination": "Dire Dawa",
      "customer_name": "Customer Name",
      "customer_phone": "+251912345678",
      "scheduled_date": "2024-01-15",
      "notes": "Fragile items",
      "created_at": "2024-01-10T10:00:00Z",
      "updated_at": "2024-01-14T08:30:00Z"
    }
  ]
}
```

---

### Scenario 3: Start Trip with Tracking

**Description**: Driver starts a trip and begins location tracking.

#### Mobile App Flow

1. Open shipment details
2. Tap "Start Trip" button
3. App requests location permissions
4. Tracking begins with adaptive location updates
5. Location updates are queued for sync

#### Expected API Calls

**Update Shipment Status**:
```
PUT /api/v1/shipments/{id}/status
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "status": "in_transit",
  "note": "Trip started"
}
```

**Sync Location Updates** (periodic or on status change):
```
POST /api/v1/sync
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "device_id": "device-uuid",
  "last_sync_at": "2024-01-15T08:00:00Z",
  "events": [
    {
      "shipment_id": "uuid",
      "latitude": 9.0320,
      "longitude": 38.7675,
      "accuracy": 10.5,
      "speed": 45.2,
      "heading": 90.0,
      "event_type": "location_update",
      "status": "in_transit",
      "recorded_at": "2024-01-15T08:30:00Z"
    }
  ],
  "battery_level": 78,
  "storage_remaining_kb": 495000
}
```

---

### Scenario 4: Update Shipment Status

**Description**: Driver updates shipment status during delivery.

#### Mobile App Flow

1. Open shipment details
2. Tap "Update Status"
3. Select new status from options:
   - In Transit
   - Delayed (with reason)
   - Arrived at destination
   - Issue encountered
4. Optionally add notes
5. Submit update

#### Available Statuses

| Status | Description |
|--------|-------------|
| `pending` | Awaiting assignment |
| `assigned` | Driver assigned |
| `in_transit` | Out for delivery |
| `delayed` | Delivery delayed |
| `arrived` | Arrived at destination |
| `delivered` | Successfully delivered |
| `issue` | Problem encountered |
| `cancelled` | Shipment cancelled |

#### Expected API Calls

```
PUT /api/v1/shipments/{id}/status
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "status": "delayed",
  "note": "Heavy traffic on route",
  "reason": "traffic"
}
```

**Delayed Reasons**:
- `road_conditions`
- `weather`
- `security_checkpoint`
- `mechanical_issue`
- `traffic`
- `other`

---

### Scenario 5: Complete POD with Signature and Photos

**Description**: Driver completes proof of delivery with recipient signature and photos.

#### Mobile App Flow

1. Open shipment details
2. Tap "Complete Delivery" or "Proof of Delivery"
3. Fill POD form:
   - Recipient name (required)
   - Recipient phone (optional)
   - Capture signature (touch pad)
   - Take delivery photos (1-5 photos)
   - Add delivery notes (optional)
4. Location verification (optional)
5. Submit POD

#### Expected API Calls

**Submit POD via Sync**:
```
POST /api/v1/sync
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "device_id": "device-uuid",
  "last_sync_at": "2024-01-15T10:00:00Z",
  "pods": [
    {
      "shipment_id": "uuid",
      "recipient_name": "Abebech Bekele",
      "recipient_phone": "+251922234567",
      "signature_data": "data:image/png;base64,iVBORw0KGgo...",
      "photo_urls": [
        "file://photos/pod_1.jpg",
        "file://photos/pod_2.jpg"
      ],
      "delivery_address": "Bole Sub-city, Addis Ababa",
      "delivery_lat": 9.0220,
      "delivery_lng": 38.7875,
      "delivery_notes": "Left with security guard",
      "recorded_at": "2024-01-15T10:30:00Z"
    }
  ],
  "statuses": [
    {
      "shipment_id": "uuid",
      "status": "delivered",
      "note": "POD completed",
      "recorded_at": "2024-01-15T10:30:00Z"
    }
  ],
  "battery_level": 65,
  "storage_remaining_kb": 480000
}
```

**Success Response**:
```json
{
  "data": {
    "server_time": "2024-01-15T10:30:05Z",
    "events_received": 2,
    "pull": {
      "shipments": [],
      "messages": [],
      "updates": []
    }
  }
}
```

---

### Scenario 6: Sync Offline Data

**Description**: Synchronize data collected while offline.

#### Mobile App Flow

1. App detects network status changes
2. When connection restored, automatic sync triggers
3. User can also manually trigger sync from settings
4. Sync progress is displayed
5. Conflicts (if any) are resolved automatically

#### Sync Flow Diagram

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Mobile App    │────▶│   Local DB      │────▶│   Backend API   │
│  (Offline)      │     │  (SQLite)       │     │   (Sync)        │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
        │  1. Save locally      │                       │
        ├──────────────────────▶│                       │
        │                       │                       │
        │  2. Queue for sync    │                       │
        ├──────────────────────▶│                       │
        │                       │                       │
        │                       │  3. Push when online  │
        │                       ├──────────────────────▶│
        │                       │                       │
        │                       │  4. Pull updates      │
        │                       │◀──────────────────────┤
        │                       │                       │
        │  5. Update UI         │                       │
        │◀──────────────────────┤                       │
```

#### Expected API Calls

```
POST /api/v1/sync
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "device_id": "device-uuid",
  "last_sync_at": "2024-01-15T08:00:00Z",
  "events": [
    {
      "shipment_id": "uuid",
      "latitude": 9.0320,
      "longitude": 38.7675,
      "accuracy": 15.0,
      "speed": 0,
      "heading": 180,
      "event_type": "location_update",
      "status": "in_transit",
      "recorded_at": "2024-01-15T09:00:00Z"
    }
  ],
  "statuses": [
    {
      "shipment_id": "uuid",
      "status": "arrived",
      "note": "Arrived at destination",
      "recorded_at": "2024-01-15T09:30:00Z"
    }
  ],
  "battery_level": 70,
  "storage_remaining_kb": 490000
}
```

**Response with Conflicts**:
```json
{
  "data": {
    "server_time": "2024-01-15T10:00:00Z",
    "events_received": 2,
    "conflicts": [
      {
        "type": "status_mismatch",
        "shipment_id": "uuid",
        "local_value": "delayed",
        "server_value": "in_transit",
        "resolution": "server_wins"
      }
    ],
    "pull": {
      "shipments": [...],
      "messages": ["Shipment ET-2024-001234 status updated by dispatcher"],
      "updates": []
    }
  }
}
```

---

## API Reference

### Authentication

All authenticated endpoints require a Bearer token:

```
Authorization: Bearer {access_token}
```

### Error Response Format

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": [
      { "field": "phone", "message": "Invalid phone format" }
    ]
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request body |
| `VALIDATION_ERROR` | 400 | Input validation failed |
| `UNAUTHORIZED` | 401 | Invalid or expired token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `ACCOUNT_LOCKED` | 403 | Account locked due to failed attempts |
| `PASSWORD_RESET_REQUIRED` | 403 | Password change required |

### Pagination

List endpoints support pagination:

```
GET /api/v1/shipments?page=1&per_page=25
```

Response includes pagination metadata:
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "per_page": 25,
    "total": 100,
    "total_pages": 4
  }
}
```

---

## Testing Checklist

### Backend

- [ ] PostgreSQL and Redis running
- [ ] Database migrations applied
- [ ] API server starts without errors
- [ ] Health endpoint returns 200
- [ ] Driver login works with valid credentials
- [ ] Sync endpoint accepts data
- [ ] My-shipments returns assigned shipments
- [ ] Driver stats endpoint works

### Frontend

- [ ] Application builds without errors
- [ ] All pages render correctly
- [ ] Login flow works
- [ ] Dashboard loads with data
- [ ] Shipment list displays
- [ ] API client connects to backend

### Mobile

- [ ] TypeScript compiles without errors
- [ ] Expo server starts
- [ ] Login screen displays
- [ ] Phone number validation works
- [ ] Login API call succeeds
- [ ] Shipments list loads
- [ ] POD form displays
- [ ] Offline storage works

---

## Troubleshooting

### Backend Won't Start

1. Check PostgreSQL is running: `docker compose ps`
2. Check migrations are applied: `make migrate-up`
3. Verify JWT keys exist in `keys/` directory
4. Check `.env` file has correct configuration

### Frontend Can't Connect to API

1. Verify backend is running on port 8080
2. Check `NEXT_PUBLIC_API_URL` in `.env.local`
3. Verify CORS is configured in backend

### Mobile App Issues

1. Check device/emulator has network access
2. Verify API URL in `app.config.ts`
3. Check Expo development server is running
4. Clear app data and re-login if auth issues

### Sync Issues

1. Check network connectivity
2. Verify access token is valid
3. Check sync queue in local database
4. Review sync conflicts in app UI

---

## Support

For issues or questions, refer to:
- Backend documentation: `/home/nate/Projects/logistics/backend/docs/`
- API Swagger docs: `http://localhost:8080/swagger/index.html`
- Project plan: `/home/nate/Projects/logistics/plan.md`