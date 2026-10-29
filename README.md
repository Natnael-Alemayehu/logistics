# Menged

**Shipment tracking and proof of delivery for Ethiopian trucking companies.**
መንገድ means "road".

A dispatcher books a load and assigns a driver. The driver's phone keeps
working with no signal: status changes, signatures, photos and GPS are saved on
the phone and sent when a network returns. The customer gets an SMS with a
tracking link when the load leaves, is delayed, arrives and is delivered, then
confirms receipt with one tap. Each delivery is recorded with the recipient's
name, their signature, up to three photos and where the phone was. A delivery
recorded far from the destination is flagged.

It is built for the road between Addis Ababa and Djibouti, not for a city with
perfect coverage. Drivers with feature phones report by SMS instead
(`ET 7K3M 3` means "arrived").

## What's in it

| For | Where | What they get |
|---|---|---|
| Dispatchers, fleet managers, admins | `/app` | Live fleet map, shipment board with tabs and filters, booking with a town list or a dropped pin, assignment, status overrides, messages to drivers, alerts (late, problem reported, driver quiet with cargo), reports with CSV export, printable waybills, team and vehicle management, an audit log, company settings and SMS preferences |
| Drivers | `/driver/` | An installable, offline-first web app in English or Amharic: today's loads, one-tap status updates, delay and problem reports with reasons, messages to the office, delivery capture (name, signature, photos, GPS) and a sync indicator that shows what is still waiting to send |
| Drivers on feature phones | SMS | `ET <last 4> <code> [note]`, codes 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem |
| Customers | `/track`, SMS links | Tracking by number or by phone (one-time SMS code), a coarse last-known position, and a confirm-or-report-a-problem page after delivery |
| The operator | `/app/platform` | Every company on the platform, plans, driver limits, suspension |

Companies sign up on their own, confirming the admin's phone with an SMS code,
and get a 14-day trial with up to 5 drivers. Staff who forget their password
reset it with an emailed link.
Data is separated per company throughout, and there are five roles: driver,
dispatcher, fleet manager, admin and platform admin.

## Architecture

One Go binary serves everything:

```
          browsers (dashboard, tracking)        driver phones (PWA)        SMS provider
                      │  HTML + WebSocket              │  JSON sync              │  webhook
                      └───────────────┬────────────────┴─────────────┬──────────┘
                                ┌─────▼──────────────────────────────▼─────┐
                                │  menged (cmd/server)                       │
                                │  internal/web      routes, templates, API  │
                                │  internal/service  business rules          │
                                │  background: SMS outbox, alerts, retention │
                                └─────────────────────┬──────────────────────┘
                                              PostgreSQL + PostGIS
```

- **Server-rendered dashboard.** `html/template` with a little vanilla JS for
  live updates, maps (Leaflet, vendored) and charts (inline SVG). No build step,
  and a strict CSP (`script-src 'self'`).
- **Driver app as a PWA** (`internal/web/static/driver`). A service worker
  caches the shell, IndexedDB holds shipments and an outbox, and `POST
  /api/v1/driver/sync` exchanges everything in one gzip-compressed request.
  Every change carries a client ID, so a resend after a lost response is
  harmless. When the phone and the office disagree, the office wins and the
  driver is told why.
- **SMS goes through an outbox** in the database, sent by a background worker
  with retries. Customers can text STOP to opt out. The mock provider logs messages to
  Settings → Text messages, where you can also simulate a driver's reply.
- **Everything is embedded**: templates, static files and migrations are in the
  binary, which migrates the database on start.

Tradeoff: a web app cannot record GPS while it is fully closed, so positions
come while the app is open or in the background on Android. Status changes,
proof of delivery and SMS reporting don't depend on GPS.

### Code map

```
backend/
  cmd/server/          main: serve, migrate, seed
  internal/web/        HTTP: routes, middleware, handlers, templates/, static/
  internal/service/    shipments, status rules, sync, auth, SMS, alerts, reports, jobs
  internal/model/      statuses, transitions, reasons, roles, plans
  internal/db/         sqlc-generated queries (from sqlc/queries)
  internal/geo/        Ethiopian towns, distances, transit-time estimates
  internal/phone/      Ethiopian phone number normalisation
  migrations/          goose migrations, embedded
  pkg/                 sms providers, storage (local or S3), websocket hub, hashing
  tests/integration/   end-to-end tests over HTTP against PostGIS
  deployments/         production compose file, Caddy, Prometheus
```

## Run it locally

You need Go 1.25+ and Docker.

```bash
make db      # PostGIS on localhost:5434 (DB_PORT=… to change)
make seed    # migrate and load a demo company
make run     # http://localhost:8080
```

The demo company, Abay Freight PLC, has six weeks of delivered loads, a few
trucks on the road and two alerts:

| Sign in as | At | With |
|---|---|---|
| Administrator | `/login` | `demo@menged.et` / `menged-demo` |
| Dispatcher | `/login` | `dispatch@menged.et` / `menged-demo` |
| Platform admin | `/login` | `admin@menged.et` / `menged-admin` |
| Driver | `/driver/` | `0912345601` … `0912345605`, PIN `2580` |

SMS is simulated by default. To try a feature-phone driver, open Settings →
Text messages and send `ET <last 4 of a tracking number> 3` from a driver's
number.

`make dev` rebuilds on Go changes (with [air](https://github.com/air-verse/air)).
Templates and static files are reloaded from disk on every request either way.

### Tests

```bash
make test               # unit tests
make test-integration   # end-to-end: signup → book → driver sync → POD → tracking → confirmation,
                        # replayed syncs, conflicts, tenant isolation, roles, SMS commands
```

The integration tests create a scratch database on the server from `make db`
and drop it afterwards. Without `TEST_DATABASE_URL` they start their own PostGIS
container instead.

## Deploy

```bash
cd backend
cp .env.example .env    # set DOMAIN, DB_PASSWORD, SMS_PROVIDER and its keys, SMS_INBOUND_SECRET
docker compose -f deployments/docker-compose.prod.yml up -d
```

This runs the app, PostGIS and Caddy, which gets a TLS certificate for `DOMAIN`.
Signatures and photos go to the `app-data` volume, or to S3 with
`STORAGE_TYPE=s3`. Add `--profile monitoring` for Prometheus and Grafana; set
`METRICS_TOKEN` and put the same value in `deployments/prometheus/metrics_token`.

Point your SMS provider's inbound webhook at
`https://DOMAIN/api/v1/sms/inbound?secret=SMS_INBOUND_SECRET`. Back up with
`scripts/backup.sh`.

Health checks: `/healthz` (process up) and `/readyz` (database reachable).

## Documents

- [`functional_requirements.md`](functional_requirements.md): the original requirements
- [`logistics.md`](logistics.md): background on the problem
- [`E2E_TEST_GUIDE.md`](E2E_TEST_GUIDE.md): a manual walkthrough of every flow
