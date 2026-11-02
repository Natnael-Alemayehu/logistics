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
| Dispatchers, fleet managers, admins | `/app` | A live fleet map with filters, route lines and a satellite view; a shipment board that updates as drivers report, with tabs, filters, saved views, sortable columns and bulk assignment; booking with a town list or a dropped pin; status overrides; messages to one driver or to every driver on the road; alerts (late, not collected, problem reported, driver quiet with cargo, moving away from the destination, delivered) with optional browser notifications and email copies; each trip's GPS history; reports by period, driver and route, with CSV and PDF; printable waybills; team and vehicle management; an audit log; company settings and SMS preferences |
| Drivers | `/driver/` | An installable, offline-first web app in English or Amharic: today's loads, one-tap status updates, delay and problem reports with reasons, messages to the office, delivery capture (name, signature, photos, GPS) with a warning away from the drop-off, distance and direction to the drop-off, battery saver, settings that follow the driver to a new phone, and a sync indicator that shows what is still waiting to send and when the signal is weak |
| Drivers on feature phones | SMS | `ET <last 4> <code> [note]`, codes 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem |
| Customers | `/track`, SMS links | Tracking by number or by phone (one-time SMS code), a coarse last-known position, and a confirm-or-report-a-problem page after delivery |
| Company admins | `/app/billing` | The plan and this month's usage, plan changes, printable invoices, payment by telebirr or CBE Birr, or bank transfer details |
| The operator | `/app/platform` | Every company with its plan and activity, trial extensions, Enterprise terms, suspension; unpaid invoices, bank transfers to record, refunds to send, monthly recurring revenue |

Companies sign up on their own, confirming the admin's phone with an SMS code,
and either start a 14-day free trial or choose a plan and pay at once. Staff
who forget their password reset it with an emailed link.
Data is separated per company throughout, and there are five roles: driver,
dispatcher, fleet manager, admin and platform admin.

## Plans

| Plan | Drivers | Texts a month | Price |
|---|---|---|---|
| Free trial | 10 | 500 | free for 14 days |
| Starter | 10 | 500 | ETB 2,500 a month |
| Business | 50 | 3,000 | ETB 9,000 a month |
| Enterprise | agreed | agreed | agreed with the operator |

Every plan has the whole product, with unlimited office staff and shipments;
plans differ only in limits. Prices exclude 15% VAT, and a year costs ten
months. Texts beyond the allowance are never blocked, because a customer who
misses a delivery update costs more than the text: each costs ETB 0.50 on the
next month's usage invoice.

A company pays for each period in advance, by bank transfer, which the
operator records, or by mobile money. There is no real telebirr or CBE Birr
adapter yet: `pkg/payments` has the interface and the stand-ins, so a
production site takes bank transfers until one is written.

An upgrade applies once its prorated invoice is paid. A smaller plan, or the
other billing cycle, starts with the next period. A plan change waits while a
renewal or usage invoice is unpaid. Invoices are due 7 days after they are
issued.

A company becomes **read-only** when its trial ends, or when a renewal or
usage invoice is still unpaid 7 days after it was due. Read-only stops new
work only: booking shipments, assigning unassigned ones, and adding drivers,
vehicles or staff. Loads already on the road carry on, with driver updates,
proof of delivery, reassignment and customer SMS. Viewing, exporting and
paying always work, and paying lifts it at once. The operator can also
suspend a company, which signs everyone out.

Money that arrives for an invoice already paid or cancelled is not kept: the
operator sees it under **Refunds to send** and records the refund once it is
sent back.

## Architecture

One Go binary serves everything:

```
          browsers (dashboard, tracking)        driver phones (PWA)        SMS provider
                      │  HTML + WebSocket              │  JSON sync              │  webhook
                      └───────────────┬────────────────┴─────────────┬──────────┘
                                ┌─────▼──────────────────────────────▼───────┐
                                │  menged (cmd/server)                       │
                                │  internal/web      routes, templates, API  │
                                │  internal/service  business rules          │
                                │  jobs: outboxes, alerts, billing, cleanup  │
                                └─────────────────────┬──────────────────────┘
                                              PostgreSQL + PostGIS
```

- **Server-rendered dashboard.** `html/template` with a little vanilla JS for
  live updates, maps (Leaflet, vendored) and charts (inline SVG). No build step,
  and a strict CSP (`script-src 'self'`). When the WebSocket reports a change,
  a page fetches itself again and swaps in its live parts, so the server
  renders everything.
- **Driver app as a PWA** (`internal/web/static/driver`). A service worker
  caches the shell, IndexedDB holds shipments and an outbox, and `POST
  /api/v1/driver/sync` exchanges everything in one gzip-compressed request.
  Every change carries a client ID, so a resend after a lost response is
  harmless. When the phone and the office disagree, the office wins and the
  driver is told why. The app asks the browser to keep its data
  (`navigator.storage.persist()`), so unsent deliveries survive a full
  phone.
- **SMS and email go through outboxes** in the database, sent by a background
  worker with retries, so a provider outage delays a message rather than
  losing it. Customers can text STOP to opt out.
- **Outside services have stand-ins.** SMS, email and payments default to
  providers that print what they would send in the server's terminal: codes,
  links, invoices and payment prompts. A production deployment refuses to
  start with them unless `ALLOW_MOCK_PROVIDERS=true`.
- **Everything is embedded**: templates, static files and migrations are in the
  binary, which migrates the database on start.

Tradeoff: a web app cannot record GPS while it is fully closed, so positions
come while the app is open or in the background on Android. The app can keep
the screen on while charging for long drives. Status changes, proof of
delivery and SMS reporting don't depend on GPS.

### Code map

```
backend/
  cmd/server/          main: serve, migrate, seed
  internal/web/        HTTP: routes, middleware, handlers, templates/, static/
  internal/service/    shipments, status rules, sync, auth, SMS, email, billing, alerts, reports, jobs
  internal/model/      statuses, transitions, reasons, roles, plans and prices
  internal/db/         sqlc-generated queries (from sqlc/queries)
  internal/geo/        Ethiopian towns, distances, transit-time estimates
  internal/phone/      Ethiopian phone number normalisation
  migrations/          goose migrations, embedded
  pkg/                 SMS, email and payment providers, terminal stand-ins, storage (local or S3),
                       websocket hub, hashing
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

SMS, email and payments are simulated by default: codes, links and payment
prompts are printed in the terminal running `make run`. To try a
feature-phone driver, open Settings → Text messages and send
`ET <last 4 of a tracking number> 3` from a driver's number, or run
`go run ./cmd/server sms-in 0912345601 "ET 7K3M 3"` from `backend/`.

`make dev` rebuilds on Go changes (with [air](https://github.com/air-verse/air)).
Templates and static files are reloaded from disk on every request either way.

### Tests

```bash
make test               # unit tests
make test-integration   # end-to-end: signup → book → driver sync → POD → tracking → confirmation,
                        # replayed syncs, conflicts, tenant isolation, roles, SMS commands,
                        # accounts, billing from trial to renewal, upgrade and refund, and
                        # dispatch tools: bulk assign, saved views, broadcasts, alert emails,
                        # route deviation, location history and report filters, and the
                        # driver app: settings across phones, deliveries away from the
                        # drop-off, suggested recipients
```

The integration tests create a scratch database on the server from `make db`
and drop it afterwards. Without `TEST_DATABASE_URL` they start their own PostGIS
container instead.

## Deploy

```bash
cd backend
cp .env.example .env    # set DOMAIN, DB_PASSWORD, SMS_* and SMTP_* settings, BILLING_BANK_DETAILS
docker compose -f deployments/docker-compose.prod.yml up -d
```

This runs the app, PostGIS and Caddy, which gets a TLS certificate for `DOMAIN`.
Signatures and photos go to the `app-data` volume, or to S3 with
`STORAGE_TYPE=s3`. Add `--profile monitoring` for Prometheus and Grafana; set
`METRICS_TOKEN` and put the same value in `deployments/prometheus/metrics_token`.

The maps' satellite view uses Esri World Imagery, which needs an ArcGIS
licence for commercial use. Point `MAP_SATELLITE_URL` at your own imagery
provider, or set it to `off`.

Point your SMS provider's inbound webhook at
`https://DOMAIN/api/v1/sms/inbound?secret=SMS_INBOUND_SECRET`. Back up with
`scripts/backup.sh`.

Health checks: `/healthz` (process up) and `/readyz` (database reachable).

## Documents

- [`functional_requirements.md`](functional_requirements.md): the original requirements
- [`logistics.md`](logistics.md): background on the problem
- [`E2E_TEST_GUIDE.md`](E2E_TEST_GUIDE.md): a manual walkthrough of every flow
