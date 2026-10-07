# Ethiopian Logistics Tracking Platform

An **offline-first logistics tracking and proof-of-delivery (POD) platform** built for the Ethiopian market. Dispatchers get real-time fleet visibility, drivers operate fully offline in rural areas with unreliable connectivity, and customers can track shipments and confirm deliveries with digital proof.

The platform tackles three core pain points in Ethiopian trucking: **no visibility** (dispatchers calling drivers all day), **no proof of delivery** (costly delivery disputes), and **customer anxiety** (no delivery status visibility). Everything is designed around Ethiopia's network reality — stable in major cities, unreliable on highways, and non-existent in rural areas.

---

## Key Features

- **Real-time GPS tracking** — live fleet map with last-known locations via WebSockets when connectivity is available
- **Offline-first tracking** — checkpoint-based status updates work without network; data syncs automatically when connection returns
- **Proof of delivery (POD)** — digital signature capture + geotagged, timestamped photo evidence with recipient verification
- **Adaptive battery-conscious GPS** — tracking frequency adjusts based on power level and movement detection
- **Background sync & conflict resolution** — offline changes queue locally (SQLite) and sync with server-side conflict resolution
- **SMS fallback** — critical status updates via SMS (Twilio or Ethio Telecom providers, mock in dev)
- **Dispatcher dashboard** — fleet overview map, shipment lifecycle, driver/vehicle management, alerts, analytics & reporting
- **Customer tracking portal** — track shipments by tracking number, SMS notifications on status changes
- **Multi-tenant & RBAC** — driver, dispatcher, and admin roles with tenant isolation

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                            BACKEND                            │
│   Go 1.25 · chi · PostgreSQL (PostGIS) · Redis · sqlc         │
│   JWT auth · WebSockets · Prometheus · Swagger · S3 · SMS     │
│        ┌──────────────┐              ┌─────────────────┐      │
│        │  logistics-api│             │ logistics-worker│      │
│        │  (REST/WS)    │             │ (async jobs)    │      │
│        └──────┬───────┘              └────────┬────────┘      │
│               └───────────────┬────────────────┘              │
│                        ┌──────▼──────┐                        │
│                        │  Postgres   │                        │
│                        │  + PostGIS  │                        │
│                        └─────────────┘                        │
└─────────────────────────────┬────────────────────────────────┘
                       REST/WS + sync
        ┌───────────────┬──────┴────────┬──────────────┐
        │               │               │              │
  ┌─────▼─────┐  ┌──────▼──────┐  ┌─────▼─────┐  ┌─────▼─────┐
  │ Driver     │  │ Dispatcher  │  │  Customer │  │  SMS      │
  │ Mobile App │  │ Web Dashboard│  │  Portal  │  │  Gateway  │
  │ (offline-  │  │ (Next.js)    │  │ (Next.js)│  │  (fallback│
  │  first)    │  │              │  │           │  │   channel) │
  └────────────┘  └──────────────┘  └───────────┘  └───────────┘
```

### Tech Stack

| Component | Directory | Stack |
|-----------|-----------|-------|
| **Backend API & Worker** | [`backend/`](backend/) | Go 1.25, [chi](https://github.com/go-chi/chi), PostgreSQL + PostGIS, Redis, [sqlc](https://sqlc.dev/), [goose](https://github.com/pressly/goose) migrations, JWT (RSA), gorilla/websocket, zerolog, Prometheus, swag/Swagger, AWS S3, testcontainers |
| **Web Frontend** | [`frontend/`](frontend/) | Next.js 16, React 19, TypeScript, Tailwind CSS v4, shadcn/ui, next-intl (i18n), Zustand, TanStack Query, react-leaflet, Recharts, react-hook-form + Zod |
| **Mobile App** | [`mobile/`](mobile/) | Expo SDK 54, React Native 0.81, expo-router, TypeScript, MapLibre, expo-sqlite, background-sync queue + conflict resolution, adaptive GPS/geofencing, i18next, Zustand, expo-notifications |

### Infrastructure & Integrations

| Service | Purpose |
|---------|---------|
| PostgreSQL + PostGIS | Primary datastore (shipments, tracking events, PODs, geo queries) |
| Redis | Rate limiting, caching, async job queue for the worker |
| AWS S3 *(optional)* | Remote photo/POD object storage (`STORAGE_TYPE=s3`, local FS fallback) |
| SMS provider *(optional)* | Twilio or Ethio Telecom (`SMS_PROVIDER`, mock in dev) |
| Grafana + Prometheus + Nginx | Monitoring stack in [`backend/deployments/`](backend/deployments/) |

---

## Monorepo Layout

```
logistics/
├── backend/                 # Go API + worker
│   ├── cmd/
│   │   ├── api/             # Application entry point (REST/WS server)
│   │   └── worker/          # Async background worker
│   ├── internal/
│   │   ├── config/          # Environment configuration
│   │   ├── db/              # sqlc-generated queries & models
│   │   ├── handler/         # HTTP handlers & route definitions
│   │   ├── middleware/      # Auth, RBAC, CORS, rate limiting, logging, metrics
│   │   ├── model/           # Domain models
│   │   └── service/         # Business logic layers
│   ├── pkg/                 # Shared packages (jwt, redis, queue, storage, sms, ...)
│   ├── migrations/          # goose SQL migrations
│   ├── sqlc/                # Query definitions & sqlc config
│   ├── tests/               # Integration tests (testcontainers) + fixtures
│   ├── deployments/         # Docker Compose + Grafana/Prometheus/Nginx configs
│   ├── docs/                # Generated Swagger docs
│   ├── scripts/             # Seed & backup scripts
│   └── docker-compose.yml   # Dev Postgres + Redis
│
├── frontend/                # Next.js web app (dispatcher dashboard, admin, customer portal)
│   ├── app/
│   │   ├── (auth)/          # Login flow
│   │   ├── (dashboard)/     # Dashboard, shipments, drivers, vehicles, reports, settings
│   │   ├── (admin)/         # Admin: tenants, users
│   │   └── track/           # Public shipment tracking portal
│   ├── components/          # UI components (shadcn/ui)
│   ├── hooks/               # Data-fetching & real-time hooks
│   ├── i18n/                # Translations
│   ├── stores/              # Zustand stores
│   └── lib/                 # Utilities & API client
│
└── mobile/                  # Expo driver app (offline-first)
    └── src/
        ├── app/             # expo-router screens (auth, shipments, map, POD, settings)
        ├── components/      # Reusable UI
        ├── hooks/           # Data hooks
        ├── services/
        │   ├── api/         # API clients
        │   ├── sync/        # Sync queue, conflict resolution
        │   ├── location/    # Adaptive tracking, geofencing, movement detection
        │   ├── websocket/   # Realtime connectivity
        │   ├── cache/       # Local caching layer
        │   └── photos/      # POD photo capture & storage
        └── db/              # Local SQLite schema
```

---

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+ (with npm)
- Docker (for Postgres + Redis, or install them natively)
- [air](https://github.com/air-verse/air) *(optional, for backend hot-reload)*

### 1. Backend

Commands must be run from the [`backend/`](backend/) directory.

```bash
# Start Postgres (PostGIS) and Redis
docker compose up -d

# Configure environment
cp .env.example .env         # then edit to match your setup

# Generate JWT RSA key pair if keys/ does not already exist
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

# Run database migrations and seed development data
make migrate-up
make seed

# Start the API server (http://localhost:8080)
make run

# Start the background worker
make run-worker

# Hot-reload development
make dev
```

Verify the API is healthy: `curl http://localhost:8080/health`

Other useful targets (see [`backend/Makefile`](backend/Makefile)): `make build`, `make test`, `make test-integration`, `make swagger`, `make sqlc`, `make docker-up`, `make docker-down`.

### 2. Frontend

Commands must be run from the [`frontend/`](frontend/) directory.

```bash
npm install

# Configure environment
cp .env.example .env.local   # set NEXT_PUBLIC_API_URL to your backend

npm run dev                  # http://localhost:3000
```

Other scripts: `npm run build`, `npm run start`, `npm run lint`, `npm run typecheck`.

### 3. Mobile

Commands must be run from the [`mobile/`](mobile/) directory.

```bash
npm install

# Configure environment (point EXPO_PUBLIC_API_URL at your backend)
cp .env.example .env

npx expo start               # then press a / i / w for Android / iOS / web
```

Other scripts: `npm run android`, `npm run ios`, `npm run web`, `npm run lint`, `npm run typecheck`.

> Run the app with Expo Go, a development build, or `npx expo run:android` for a native build. Note: `useMaplibreNative()` routes (live map) require a development build rather than Expo Go — see [`mobile/src/services/maps/native.ts`](mobile/src/services/maps/native.ts).

---

## Testing

| Command | Scope |
|---------|-------|
| `make test` | Unit tests (`-short`, no external services) |
| `make test-integration` | Integration tests via testcontainers (boots real Postgres/Redis in Docker). Customize timeout: `make test-integration TIMEOUT=1h` |
| `make test-all` / `make ci` | Unit + integration suite |
| `npm run lint` + `npm run typecheck` | Frontend & mobile static checks |

See [`E2E_TEST_GUIDE.md`](E2E_TEST_GUIDE.md) for the end-to-end test workflow covering backend, frontend, and mobile.

---

## API & Observability

- **Swagger UI**: `http://localhost:8080/swagger/` (generated via `make swagger`)
- **Health check**: `GET /health`
- **Prometheus metrics**: `GET /metrics`
- **Realtime**: WebSocket endpoint `GET /ws` (JWT-authenticated) for live tracking events

Main API surface (under `/api/v1`, all authenticated except login and tracking):

| Area | Endpoints |
|------|-----------|
| Auth | `POST /auth/login/driver`, `/auth/login/dispatcher`, `/auth/refresh`, `/auth/logout`, `/auth/change-password`, `/auth/forgot-pin` |
| Public tracking | `GET /track/{tracking_number}` |
| Sync | `POST /sync` (offline delta sync) |
| Shipments | `GET/POST/PUT /shipments`, `GET /shipments/search`, status updates, assignment, cancellation, tracking events, POD |
| Drivers | `GET/POST /drivers`, `GET /drivers/locations`, per-driver location |
| Vehicles | `GET/POST/PUT /vehicles`, active vehicles |
| Dashboard | `GET /dashboard/stats`, `/alerts`, `/activity` |
| Users (admin) | `GET/POST/PUT /users` |
| Notifications | `POST /notifications/register`, `DELETE /notifications/device/{deviceId}` |
| Sessions | `GET /sessions`, revoke by id or all others |

---

## Configuration

Each component is configured via environment variables; reference the `.env.example` files:

- **Backend**: [`backend/.env.example`](backend/.env.example) — server port, database URL, Redis, JWT keys/TTLs, storage (local/S3), SMS provider, tracking URL base
- **Frontend**: [`frontend/.env.example`](frontend/.env.example) — API/WebSocket URLs, map tile URL, default locale, app name
- **Mobile**: [`mobile/.env.example`](mobile/.env.example) — API base URL

---

## Key Documents

| Document | Description |
|----------|-------------|
| [`plan.md`](plan.md) | Master development plan — consult before implementing features |
| [`functional_requirements.md`](functional_requirements.md) | Functional requirements |
| [`non_functional_requirements.md`](non_functional_requirements.md) | Non-functional requirements |
| [`frontend-plan.md`](frontend-plan.md) | Frontend implementation plan |
| [`mobile-plan.md`](mobile-plan.md) | Mobile implementation plan |
| [`E2E_TEST_GUIDE.md`](E2E_TEST_GUIDE.md) | End-to-end testing workflow |

Also see [`AGENTS.md`](AGENTS.md) for agent/developer notes on working with this codebase. Backend, frontend, and mobile must be worked on from within their respective directories.

---

## Repository

Source: [github.com/Natnael-Alemayehu/logistics](https://github.com/Natnael-Alemayehu/logistics)
Feedback: [website](natnaelalemayehu.com)
