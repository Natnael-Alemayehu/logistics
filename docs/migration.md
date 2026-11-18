# Migration: from one Go application to three codebases

How to get from today's Kiflet (one Go binary that renders the dashboard,
the public pages and the driver PWA) to the architecture in
`docs/architecture.md`: a modular Go backend with RabbitMQ and Redis in
this repository, a Next.js web app in **kiflet-web**, and a React Native
(Expo) driver app in **kiflet-mobile**.

It is written for whoever does the work, step by step, with what each step
produces, how to check it, and how to undo it.

Contents:

1. [Principles](#1-principles)
2. [Where things are today](#2-where-things-are-today)
3. [The end state](#3-the-end-state)
4. [Where everything goes](#4-where-everything-goes)
5. [The phases](#5-the-phases)
   - [Phase 0: groundwork](#phase-0-groundwork)
   - [Phase 1: modules, in place](#phase-1-modules-in-place)
   - [Phase 2: infrastructure: RabbitMQ, Redis, separate processes](#phase-2-infrastructure-rabbitmq-redis-separate-processes)
   - [Phase 3: the web API and its contract](#phase-3-the-web-api-and-its-contract)
   - [Phase 4: kiflet-web](#phase-4-kiflet-web)
   - [Phase 5: kiflet-mobile](#phase-5-kiflet-mobile)
   - [Phase 6: removing the old](#phase-6-removing-the-old)
6. [Database changes](#6-database-changes)
7. [Testing through the migration](#7-testing-through-the-migration)
8. [Order, overlap and size](#8-order-overlap-and-size)
9. [Risks](#9-risks)
10. [Still to decide](#10-still-to-decide)

---

## 1. Principles

1. **No big bang.** The current app keeps serving customers the whole
   time. New parts replace old ones one route, one page, one worker at a
   time behind Caddy (the "strangler fig" pattern). At every step `main`
   can be deployed.
2. **One database.** All three codebases use the same PostgreSQL through
   the Go API. There is no data migration between systems, only schema
   changes in place, each backwards-compatible with the version running.
3. **The backend first.** The web and mobile apps need an API to talk to;
   the API needs the modules. So the order is modules, then infrastructure,
   then the API, then the clients. The clients' bootstrapping (repository,
   design system, tooling) can start in parallel.
4. **Parity before removal.** A page or feature is removed from the Go app
   only when the new one does everything it did, checked against
   `docs/E2E_TEST_GUIDE.md` and the tests.
5. **The rules stay in Go.** Business rules (statuses and transitions,
   plan limits, alert conditions, what a role may do) are not rewritten in
   TypeScript. The clients ask the API, as the driver PWA already does with
   `/api/v1/driver/config`.
6. **Same conventions as now.** Tenant scoping in every query; append-only
   events and audit log; words customers read in English and Amharic;
   metrics labels from short fixed lists; no secrets in the repositories;
   `CHANGELOG.md` for every change.

---

## 2. Where things are today

At `main` on 2026-11-18:

- **One Go module** (`github.com/natnael-alemayehu/kiflet`), one binary
  (`cmd/server`) that serves everything and runs the background workers in
  goroutines (`internal/app.Run`: live relay, SMS, mail, webhooks, jobs).
- **`internal/service`**: all business rules, ~14,000 lines in 40 files,
  one `Services` struct; **`internal/db`**: sqlc over 34 tables in one
  schema; **`internal/model`**: statuses, transitions, roles, plans.
- **`internal/web`**: chi routes (`server.go`), ~96 routes rendering
  `html/template` pages (53 templates), JSON for the driver app
  (`/api/v1/driver/*`), the public API (`/api/v1/shipments`, `/drivers`),
  provider callbacks (SMS, telebirr, CBE Birr), the WebSocket (`/app/live`).
- **The driver app**: a PWA in `internal/web/static/driver`, 20 ES modules,
  IndexedDB for its offline store, a service worker, `POST
  /api/v1/driver/sync` as its only data call.
- **Background work**: an outbox in tables (`sms_messages`, `emails`,
  `webhook_deliveries`) polled by workers; `Jobs.Run` every minute under a
  Postgres advisory lock (driver, ETA, pickup, route and stop checks,
  billing, plan limits, papers, retention); live updates across instances
  by `LISTEN`/`NOTIFY` (`internal/app/live.go`).
- **Infrastructure**: Docker Compose (`deployments/`), Caddy, PostGIS 15,
  Prometheus and Grafana, S3 storage (SeaweedFS in staging), backups with
  point-in-time restore; staging over HTTPS with the provider sandbox.
- **Tests**: 175 unit, ~100 integration (against PostGIS), 9 end-to-end
  against staging (one drives the driver PWA in headless Chrome).

What already fits the target and must be kept:

- The driver sync protocol: one request, many items, idempotent by
  `client_id`, a cursor. The Expo app uses the same shape.
- The outbox tables and retry schedules for SMS, email and webhooks; they
  become the source for RabbitMQ messages rather than being replaced.
- The provider adapters (`pkg/sms`, `pkg/payments`) and the sandbox.
- The public API, its OpenAPI file and `TestOpenAPIMatchesTheAPI`.
- The design (Desk and Night tokens, Geist, lucide), the words in
  `internal/text`, the E2E guide.

---

## 3. The end state

| Repository | Contains | Deploys as |
|---|---|---|
| **kiflet** (this one) | Go backend: modules, `cmd/{api,realtime,worker,scheduler,relay,migrate,sandbox}`; migrations; OpenAPI and AsyncAPI contracts; the generated `@kiflet/api` TypeScript client package; `deployments/` for every environment (Caddy, Postgres, PgBouncer, RabbitMQ, Redis, storage, monitoring); integration tests; the docs | Docker images `kiflet`, `kiflet-sandbox`; the `@kiflet/api` npm package |
| **kiflet-web** | Next.js: the public pages, the dashboard, the operator's pages; the design system; Playwright tests | Docker image `kiflet-web` |
| **kiflet-mobile** | Expo app for drivers: SQLite, sync engine, background GPS, screens; Maestro tests | Play Store / App Store builds (EAS Build), over-the-air updates (EAS Update), an APK on the website |

This repository stops serving HTML. Its only HTTP surfaces are `/api/*`,
`/ws`, `/healthz`, `/readyz` and `/metrics`.

---

## 4. Where everything goes

### Directories

| Today | Goes to | Notes |
|---|---|---|
| `cmd/server` | `cmd/api`, `cmd/worker`, `cmd/scheduler`, `cmd/relay`, `cmd/realtime`, `cmd/migrate` | `seed` and `sms-in` stay as subcommands of a `cmd/kiflet` tool |
| `cmd/sandbox`, `internal/sandbox` | stay | |
| `internal/app` | `internal/app` | builds each process from its settings, as now |
| `internal/config` | `internal/platform/config` | gains `RABBITMQ_URL`, `REDIS_URL`, `WEB_ORIGIN` |
| `internal/db` | per module `internal/modules/*/store` | sqlc with one config per module |
| `internal/geo`, `internal/phone` | `internal/platform/geo`, `internal/platform/phone` | |
| `internal/model` | each module's `domain/` | transitions to `shipments/domain`, roles to `identity/domain`, plans to `billing/domain` |
| `internal/service/*.go` | `internal/modules/*/app` | file by file, table below |
| `internal/text` | stays for SMS and email (`internal/platform/text`); page words copied into kiflet-web's message files | the parity test moves with them |
| `internal/web/server.go`, middleware | `internal/http`: router, auth, CSRF, rate limits, request IDs | |
| `internal/web/api_driver.go` | `internal/modules/*/http` under `/api/driver` | |
| `internal/web/api_public.go` | `internal/modules/*/http` under `/api/v1` | unchanged contract |
| `internal/web/app_*.go`, `public.go` | JSON handlers under `/api/web`; the pages go to kiflet-web | |
| `internal/web/templates/**` | kiflet-web `app/**` | page table below; deleted here in phase 6 |
| `internal/web/static/app.css`, `app.js`, `map.js`, fonts, icons | kiflet-web `styles/`, `components/` | tokens to Tailwind; `map.js` to a MapLibre component |
| `internal/web/static/driver/**` | kiflet-mobile, rewritten | kept here until the app has replaced it (phase 5) |
| `internal/web/static/openapi.json` | `api/openapi/v1.yaml` | |
| `pkg/sms`, `pkg/payments`, `pkg/mail`, `pkg/storage` | `internal/platform/*` or stay in `pkg` | they are adapters, no rules |
| `pkg/websocket` | `cmd/realtime` | |
| `pkg/metrics`, `pkg/hash`, `pkg/console` | stay | |
| `migrations` | stay | new migrations for schemas, outbox, partitions, push tokens |
| `deployments` | stay, grow | one compose file per VM role (section 13 of the architecture) |
| `tests/integration` | stay | run against the new processes |
| `tests/e2e` | split: API and infrastructure tests stay; browser tests to kiflet-web (Playwright); driver app tests to kiflet-mobile | |
| `docs` | stay | E2E guide updated per phase |

### Services to modules

| `internal/service` file(s) | Module |
|---|---|
| `auth.go`, `accounts.go`, `users.go`, `verification.go` | identity (staff and drivers' accounts), fleet (driver profile fields) |
| `tenants.go`, settings parts of `notify.go` | tenancy |
| `vehicles.go`, `papers.go` | fleet |
| `shipment.go`, `shipment_status.go`, `shipment_files.go`, `shipment_import.go`, `dispatch.go`, `convert.go` | shipments |
| `pod.go`, `media.go` | proof |
| `tracking.go`, `sync.go` (positions part) | tracking |
| `sync.go` (the rest) | a `driversync` façade that calls shipments, proof, messaging and tracking |
| `alerts.go`, `alert_rules.go`, the checks in `jobs.go` | alerts |
| `sms.go`, `mail.go`, `notify.go` | notify |
| broadcasts and driver messages (in `dispatch.go` and `sync.go`; the messages are rows in `shipment_events`) | messaging, writing through shipments |
| `shipment_public.go` | customer |
| `billing*.go` | billing |
| `api.go`, `api_json.go`, `webhooks.go` | integrations |
| `reports.go` | reports |
| `support.go` | support |
| `audit.go` | audit |
| `data.go`, retention in `jobs.go` | data |
| `health.go` | platform (operator) |
| `jobs.go` | scheduler (the timer) and each module's job handlers |

### Pages to kiflet-web

| Today (route → template) | kiflet-web route |
|---|---|
| `/` → `public/landing.html` | `app/(public)/page.tsx` |
| `/login`, `/signup`, `/signup/verify`, `/forgot`, `/reset/*`, `/verify-email/*` | `app/(public)/login`, `signup`, `signup/verify`, `forgot`, `reset/[token]`, `reset/phone`, `verify-email/[token]` |
| `/track`, `/track/{number}`, `/track/phone`, `/track/mine` | `app/(public)/track`, `track/[number]`, `track/phone`, `track/mine` |
| `/c/{token}` → `confirm.html` | `app/(public)/c/[token]` |
| `/app` → `dashboard.html` | `app/(dashboard)/app/page.tsx` |
| `/app/shipments`, `/new`, `/import`, `/{id}`, `/{id}/edit`, `/{id}/waybill` | `app/(dashboard)/app/shipments/…` (waybill as a print page) |
| `/app/drivers`, `/new`, `/{id}`, `/{id}/edit`, `/message` | `app/(dashboard)/app/drivers/…` |
| `/app/vehicles`, `/new`, `/{id}`, `/{id}/edit` | `app/(dashboard)/app/vehicles/…` |
| `/app/alerts`, `/alerts/recipients` | `app/(dashboard)/app/alerts/…` |
| `/app/reports` | `app/(dashboard)/app/reports` |
| `/app/team`, `/new`, `/{id}/edit` | `app/(dashboard)/app/team/…` |
| `/app/settings`, `/settings/api`, `/settings/webhooks/*`, `/settings/sms` | `app/(dashboard)/app/settings/…` |
| `/app/billing`, `/billing/invoices/{id}`, `/billing/payments/{id}` | `app/(dashboard)/app/billing/…` |
| `/app/help`, `/help/new`, `/help/{id}` | `app/(dashboard)/app/help/…` |
| `/app/account`, `/account/password` | `app/(dashboard)/app/account/…` |
| `/app/audit` | `app/(dashboard)/app/audit` |
| `/app/platform/*` | `app/(operator)/platform/…` |
| `/dev/payments/*`, `/dev/cbebirr` | `app/(dev)/…`, built only when `NEXT_PUBLIC_DEV_PAYMENTS=true` |

### Driver PWA modules to kiflet-mobile

| `static/driver/` | kiflet-mobile |
|---|---|
| `app.js`, `router.js` | `app/_layout.tsx`, expo-router routes |
| `screens/login.js`, `pin.js` | `app/(auth)/login.tsx`, `pin.tsx` |
| `screens/list.js` | `app/(app)/index.tsx` |
| `screens/detail.js` | `app/(app)/shipments/[id].tsx` (react-native-maps or MapLibre) |
| `screens/deliver.js`, `signature.js` | `app/(app)/shipments/[id]/deliver.tsx`, `react-native-signature-canvas`, `expo-camera` / `expo-image-picker` |
| `screens/report.js` | `app/(app)/shipments/[id]/report.tsx` |
| `screens/message.js` | `app/(app)/messages.tsx` |
| `screens/menu.js`, `settings.js` | `app/(app)/menu.tsx`, `src/settings` |
| `store.js` (IndexedDB) | `src/db` (expo-sqlite + Drizzle schema and migrations) |
| `sync.js` | `src/sync` (outbox, cursor, conflict handling, backoff) |
| `gps.js` | `src/location` (expo-location background task, foreground service) |
| `api.js` | `src/api` (the generated `@kiflet/api` client) |
| `text.js` | `src/i18n/en.json`, `am.json` (i18next) |
| `state.js` | Zustand store |
| `sw.js`, `build.json` | EAS Update (no service worker) |

---

## 5. The phases

Each phase lists its steps, what done looks like, and how to roll back.

### Phase 0: groundwork

Decisions and accounts, before code moves.

1. **Repositories.** Create `kiflet-web` and `kiflet-mobile` (private or
   public, same licence as this one). Decide first whether to create the
   GitHub organization `kiflet` (ROADMAP, "No GitHub organization yet"):
   three repositories under one organization share secrets, packages and
   permissions more easily, and moving all three later is more work than
   moving one.
2. **Accounts.** Expo (EAS Build and Update), Google Play Console, Apple
   Developer Program (when iOS is wanted), Firebase project for FCM (push
   on Android), GitHub Packages for `@kiflet/api`.
3. **Domains.** `app.kiflet.et` (dashboard), `track.kiflet.et` or
   `kiflet.et/track` (public pages; the short form matters in SMS),
   `api.kiflet.et` (public API, same backend). Today everything is one
   host; tracking links already sent by SMS must keep working (redirects).
4. **Contracts tooling** (in this repository):
   - OpenAPI 3.1 files under `api/openapi/` (`web.yaml`, `driver.yaml`,
     `v1.yaml`).
   - Go: `oapi-codegen` generating request and response types and a
     strict server interface per API, which the module handlers implement.
   - TypeScript: `openapi-typescript` for types and `openapi-fetch` for the
     client, packaged as `@kiflet/api` with a version per release of this
     repository.
   - CI: lint the specs, regenerate and fail on a diff (as `sqlc diff`
     does), publish the package on a tag.
5. **Conventions written down** in each new repository's `AGENTS.md` and
   `CONTRIBUTING.md`: the same rules (tenant first, text in both
   languages, no inline scripts, CHANGELOG), plus their own (TypeScript
   strict, ESLint, Prettier, no `any`).

Done when: both repositories exist with CI that builds an empty app, and
this repository's CI publishes a first `@kiflet/api` from the public API's
spec.

Roll back: nothing to roll back; no code has moved.

### Phase 1: modules, in place

Reorganise this repository into the modules of the architecture without
changing behaviour. The server-rendered dashboard and the PWA keep working
throughout; only imports move.

1. **Platform packages first:** move `config`, `geo`, `phone`, `text`,
   `storage`, `mail`, `sms`, `payments` under `internal/platform/` (or keep
   `pkg/` where it is, see ROADMAP "Considered and not planned"). One
   commit, `go build ./...` and all tests green.
2. **One module at a time**, in this order (fewest dependencies first):
   audit → tenancy → identity → fleet → shipments → proof → tracking →
   messaging → customer → notify → alerts → integrations → billing →
   reports → support → data. For each:
   1. Create `internal/modules/<name>/` with `<name>.go` (the interface),
      `app/`, `domain/`, `store/`.
   2. Move its `service` code into `app/`, its rules from `model` into
      `domain/`, its queries from `sqlc/queries` into `store/queries/`, with
      an sqlc config per module (`sqlc.yaml` gains one entry per module).
   3. Replace calls from other modules into its internals with calls
      through its interface. Where a module would call one that depends on
      it, publish an event instead (in this phase, an in-process event bus
      with the same interface the RabbitMQ one will have).
   4. Move its handlers from `internal/web/app_*.go` next to it under
      `http/` (still rendering templates for now).
   5. `make test test-integration`; one PR per module.
3. **A test that enforces the boundaries:** `go list -deps` per module;
   a module may import only `internal/platform/*`, its own packages and
   other modules' root packages (their interfaces), never their `app/` or
   `store/`. Like the sandbox's import test today.
4. **Events, in process:** an `events.Bus` interface (`Publish(ctx, tx,
   Event)`, `Subscribe(type, handler)`) with an in-memory implementation
   that runs handlers after the transaction commits. Alerts, notify,
   webhooks and live updates subscribe to it instead of being called
   directly.

Done when: `internal/service`, `internal/model` and `internal/db` are gone;
every module has the layout of the architecture's section 4; the boundary
test passes; nothing a user sees has changed.

Roll back: each module is one PR; revert it.

### Phase 2: infrastructure: RabbitMQ, Redis, separate processes

1. **Compose files.** Add RabbitMQ (`rabbitmq:4-management`, quorum
   queues) and Redis (`redis:8`, AOF) to `deployments/dev/compose.yml`,
   staging and production; PgBouncer in front of Postgres. Definitions
   (exchanges, queues, policies) in `deployments/rabbitmq/definitions.json`,
   loaded at start. `make db` becomes `make deps` (Postgres, RabbitMQ,
   Redis).
2. **Postgres 15 → 17** (PostGIS 3.5): a dump and restore in staging, then
   production with `pg_upgrade --link` in a maintenance window, or a
   logical replica and a switch. Tested by staging's restore test.
3. **The broker package** (`internal/platform/broker`): connection with
   reconnects, channels per goroutine, publisher confirms, the envelope,
   consumers with prefetch, ack after commit, retries through delayed
   dead-letter queues, metrics per queue. The `events.Bus` from phase 1
   gets a RabbitMQ implementation.
4. **The outbox:** migration for `outbox` (id, type, version, tenant_id,
   routing_key, payload jsonb, created_at, sent_at), written in the same
   transaction as each change; `cmd/relay` publishes and marks sent
   (`LISTEN outbox` to wake, `FOR UPDATE SKIP LOCKED`, leader by advisory
   lock); `processed_messages` for consumers' idempotency.
5. **Move the workers to queues, one at a time,** each behind a setting
   so it can be switched back:
   1. Webhooks (`webhook.deliver`), keeping `webhook_deliveries` as the
      record and its retry schedule.
   2. Email (`notify.email`).
   3. SMS (`notify.sms`), rate-limited by prefetch to the gateway's limit.
   4. Alerts evaluation from events (`alerts.evaluate`).
   5. Audit writes (`audit.write`).
   6. Live updates: `kiflet.live` exchange replaces `LISTEN`/`NOTIFY`
      (`internal/app/live.go`).
   7. Timed jobs: `cmd/scheduler` publishes to `jobs.scheduled` on the
      schedule `Jobs.Run` keeps today; the jobs worker runs them.
6. **Positions through the queue:** the sync handler publishes positions
   to `tracking.ingest` (with confirms) instead of inserting them; the
   tracking worker batch-inserts, updates `driver_positions` and Redis,
   runs the stop, route and arrival checks, publishes events. The response
   to the phone keeps its shape (`locations_stored` becomes "accepted").
7. **Partition `tracking_events`** by day (section 6 of this document).
8. **Redis:** session, driver-token and API-key lookups cached; rate
   limits in Redis (Caddy's sticky routing becomes unnecessary); latest
   positions and presence for the fleet map; tenant data cache with
   deletion on change.
9. **Separate processes:** `cmd/api` (HTTP), `cmd/realtime` (WebSocket),
   `cmd/worker --queues=tracking.*` and so on, `cmd/scheduler`,
   `cmd/relay`, `cmd/migrate`. `cmd/server` stays for one release as "all
   of them in one process" for development and small installs.
10. **Load test** in staging (k6): 12,000 simulated phones syncing every
    60 s, then 3,000 reconnecting with 2 hours of positions each; the
    targets in the architecture's section 15 (oldest message in
    `tracking.ingest` under 2 minutes, API p95 under 300 ms).

Done when: production runs the separate processes on RabbitMQ and Redis;
the load test passes; stopping Redis or one RabbitMQ node in staging loses
nothing (tested); the old in-process workers are deleted.

Roll back: each worker move has a setting (`WORKER_<NAME>=inprocess|queue`)
until it has run in production for two weeks; Redis can be turned off
(`REDIS_URL` empty falls back to Postgres and per-instance limits).

### Phase 3: the web API and its contract

The dashboard today renders HTML from handlers. kiflet-web needs the same
data and actions as JSON. Build `/api/web` module by module, in the order
the pages will move (phase 4).

1. **For each page in the page table**, list what it reads and what forms
   it posts; that is its endpoints. For example the shipment page:
   - `GET /api/web/shipments/{id}` (shipment, driver, vehicle, customer,
     progress, POD summary)
   - `GET /api/web/shipments/{id}/events` (the activity)
   - `GET /api/web/shipments/{id}/trail` (exists as
     `/app/shipments/{id}/trail` since PR #46)
   - `GET|POST|DELETE /api/web/shipments/{id}/documents…`,
     `…/charges…`
   - `POST /api/web/shipments/{id}/status|assign|unassign|cancel|reopen|note`
   - `POST /api/web/shipments/{id}/pods/{pod}/accept|dismiss`
2. **Write the spec first** in `api/openapi/web.yaml`, generate, implement
   the handler by calling the module's use case (the same one the HTML
   handler calls), and test it in `tests/integration` (status codes,
   tenant isolation, roles, validation errors).
3. **Auth for the web:** the session cookie set by `POST /api/web/session`
   (sign in), `DELETE /api/web/session` (sign out), `GET /api/web/me` (who,
   tenant, role, what they may do, feature flags). CSRF: double-submit
   token in a header (`X-CSRF-Token`) checked by the API.
4. **What the server decides, the API says.** Each response carries what
   the user may do with it (`"actions": ["assign", "cancel"]`) so the web
   doesn't reimplement roles or transitions.
5. **Lists:** cursor pagination, filters and sort as query parameters,
   the same filters as today's shipment list and saved views.
6. **Errors:** one shape, field errors keyed by field name, messages in
   the user's language.
7. **Publish** `@kiflet/api` with each tagged release.

Done when: every page in the page table has its endpoints, specified,
implemented and tested; the HTML handlers still work (they can call the
same use cases).

Roll back: additive only; nothing uses the new endpoints until phase 4.

### Phase 4: kiflet-web

1. **Bootstrap:** Next.js (App Router), TypeScript strict, pnpm, ESLint,
   Prettier, Vitest, Playwright, Tailwind, shadcn/ui, next-intl, MapLibre
   GL, `@kiflet/api`. Dockerfile (standalone output) and CI (lint, types,
   unit, build, Playwright against a backend in Docker Compose).
2. **The design system:** port the Desk and Night tokens from
   `internal/web/static/app.css` (colours, radius, shadows, type) to the
   Tailwind theme with the same names; Geist fonts; lucide icons (the
   names in `internal/web/icons.go`); the components the templates use
   (shell with sidebar, cards, badges with the status colours, tables,
   forms, segmented controls, switches, KPIs, toasts, empty states). Check
   them side by side with today's pages in both themes.
3. **Auth and the shell:** sign-in, sign-out, the session from `GET
   /api/web/me` in a server component, the sidebar by role, the theme, the
   language, the live socket (`/ws`) as a client provider that tells pages
   what changed (as `data-live-region` does now).
4. **Pages, in this order**, each one cut over in Caddy as soon as it
   reaches parity (Caddy sends that path to Next.js, the rest stays on Go):
   1. Public tracking (`/track/*`) and confirmation (`/c/*`): small,
      read-only, the most visited, and they prove SSR, i18n and small pages
      on phones.
   2. Sign-in, sign-up, password reset.
   3. Read-mostly dashboard pages: reports, audit, help, account.
   4. Drivers, vehicles, team (forms and lists).
   5. Shipments: list, booking, import, a shipment's page, waybill.
   6. The overview: map, KPIs, needs attention, live updates.
   7. Alerts and rules; settings, API keys, webhooks, SMS log; billing.
   8. The operator's pages.
5. **Parity for each page:** the E2E guide's steps for it pass on the new
   page; Playwright tests written for them; both themes; English and (for
   public pages) Amharic; a phone-width check; the CSP holds.
6. **Links in old SMS** keep working: Caddy keeps `/track/{number}` and
   `/c/{token}` on the same paths.
7. **Remove each Go template** once its page has served production for a
   week with no regressions, in a PR of its own.

Done when: Caddy sends every non-API path to Next.js; the Go app serves no
HTML.

Roll back: per page, in Caddy: point the path back at the Go app (whose
template is still there until step 7).

### Phase 5: kiflet-mobile

Can start once phase 2's positions-through-the-queue and phase 3's
contract tooling are done; the driver API changes are small.

1. **Driver API changes** (this repository, `/api/driver`, keeping
   `/api/v1/driver/*` working for the PWA):
   - `POST /api/driver/uploads` returns presigned PUT URLs for photos and
     signatures; `SyncPOD` accepts storage keys instead of inline data
     (inline still accepted for the PWA).
   - `POST /api/driver/push-token` (Expo push token, platform); a
     `push_tokens` table; notify sends push for new loads, changes and
     messages.
   - `X-App-Version` header; `426 Upgrade Required` below the minimum
     supported version, set in config.
   - The sync request and response stay the same shape, specified in
     `api/openapi/driver.yaml`.
2. **Bootstrap:** Expo (managed workflow, development builds), TypeScript
   strict, expo-router, expo-sqlite with Drizzle, TanStack Query, Zustand,
   i18next, expo-location, expo-task-manager, expo-background-fetch,
   expo-camera, expo-image-manipulator (compress photos), expo-secure-store,
   expo-notifications, react-native-maps or MapLibre, Sentry. EAS Build
   profiles: development, preview (internal testers), production.
3. **The local database:** tables `shipments`, `shipment_stops`, `outbox`,
   `positions`, `messages`, `files` (local path, upload state, storage
   key), `settings`, `meta` (cursor, last sync, driver profile). Drizzle
   migrations at start.
4. **The sync engine:** reads the outbox and positions, uploads files
   first, sends one sync, applies the response (shipments changed and
   removed, messages, profile, cursor), deletes what the server confirmed,
   backs off on errors, and runs on change, every 60 s in the foreground,
   on reconnect (NetInfo), on a push, and in background fetch.
5. **GPS:** the background location task as an Android foreground
   service, the same intervals and battery saver as `gps.js`, writing
   only to SQLite; started when the driver has an active load and sharing
   is on, stopped after the last delivery. The permission flow explained in
   both languages, as Play's policy for background location requires
   (a prominent disclosure and a declaration in the Play Console).
6. **Screens:** port each PWA screen (table above), at the same large tap
   sizes, with the Desk and Night colours.
7. **Tests:** unit tests for the sync engine and database (Jest);
   end-to-end flows on an emulator with Maestro: sign in, depart, go
   offline, deliver with photo and signature, come back online, see it on
   the dashboard (against staging).
8. **Pilot:** internal testing track with a few drivers on their own
   phones (cheap Android, weak signal, screen locked; the ROADMAP's
   "session on a cheap Android phone"), then a closed track for one
   company, then production. The PWA stays available throughout.
9. **Move drivers over:** the dashboard shows each driver's app (PWA or
   Expo, and version); an SMS invites PWA drivers to install the app; the
   PWA shows a banner. When no active driver has used the PWA for 30 days,
   phase 6 removes it.

Done when: every active driver uses kiflet-mobile, and positions keep
coming with the screen locked.

Roll back: drivers can sign in to the PWA again at any time until it is
removed; the API serves both.

### Phase 6: removing the old

1. Delete `internal/web/templates`, the HTML handlers, `static/` (the
   dashboard's CSS and JS, the PWA), `internal/web/driver.go`, and the
   tests that rendered pages (`go test ./internal/web/` parsing templates;
   `TestDriverBuild`).
2. Delete `cmd/server`'s all-in-one mode if no installation uses it, or
   keep it documented for development.
3. Drop what only the old code used: inline POD data in sync,
   `/api/v1/driver/*` (after the last PWA driver), the `LISTEN`/`NOTIFY`
   relay.
4. Update `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `docs/deploy.md`,
   `docs/E2E_TEST_GUIDE.md` (now spanning three repositories), and record
   the change in `CHANGELOG.md`. Delete this document's finished phases,
   as ROADMAP does.

---

## 6. Database changes

All migrations are goose files here, rerunnable, and backwards-compatible
with the version deployed before them.

| Change | Phase | How, without downtime |
|---|---|---|
| Module schemas (`shipments`, `tracking`…) | 1, at the end | `ALTER TABLE … SET SCHEMA` is instant; for one release set `search_path` to include every schema so old and new code both find the tables; then qualify names in queries and drop the `search_path` change |
| `outbox`, `processed_messages` | 2 | new tables; nothing reads them until the relay starts |
| `tracking_events` partitioned by day | 2 | create `tracking.tracking_events_p` partitioned, with partitions for the last 90 days and the next 7; write new positions to it (the tracking worker); copy older rows in batches by day in a background job; when the copy is done, swap names in one transaction and keep the old table for a week before dropping it. Retention then drops partitions. |
| `driver_positions` GiST index, BRIN on `recorded_at` | 2 | `CREATE INDEX CONCURRENTLY` |
| Row-level security policies | 2 or 3 | add policies with `FORCE ROW LEVEL SECURITY` off, set `app.tenant_id` in every transaction, check in staging that nothing is refused, then turn it on table by table |
| `push_tokens`, app version per device | 5 | new table, new column with a default |
| Drop inline POD columns or fields, the PWA's tables if any | 6 | after the last PWA driver |
| Postgres 15 → 17 | 2 | staging first, then production (phase 2, step 2) |

The append-only triggers on `shipment_events` and `audit_logs` move with
their tables and keep working; `purgeTx` stays in the data module.

---

## 7. Testing through the migration

| Level | Where | What changes |
|---|---|---|
| Unit | each repository | Go modules keep theirs; TypeScript code gets Vitest (web) and Jest (mobile) |
| Integration | this repository, `tests/integration` | grows with `/api/web` and `/api/driver` endpoints; runs the separate processes with RabbitMQ and Redis in Docker; adds failure tests: Redis stopped, a RabbitMQ node stopped, a consumer crashing mid-batch (no loss, no duplicates) |
| Contract | this repository's CI | the OpenAPI specs match the handlers (as `TestOpenAPIMatchesTheAPI` now, for all three APIs); generated code has no diff |
| Browser end-to-end | kiflet-web, Playwright | the E2E guide's web sections, against a backend started from this repository's compose files at a pinned image tag |
| App end-to-end | kiflet-mobile, Maestro | the E2E guide's driver sections, on an Android emulator, against staging |
| Staging | all three | staging runs the backend images, the web image and a preview build of the app; this repository's `tests/e2e` keeps the backup restore and monitoring tests |
| Load | staging, k6 | phase 2's load test, run again before each phase's cut-over |

Each repository's CI runs its own tests on every PR. A change across
repositories (a new endpoint and the page that uses it) lands in this
repository first, releases `@kiflet/api`, then the web or mobile PR bumps
the package.

---

## 8. Order, overlap and size

```mermaid
flowchart LR
  P0["Phase 0<br/>groundwork"] --> P1["Phase 1<br/>modules in place"]
  P1 --> P2["Phase 2<br/>RabbitMQ, Redis,<br/>processes"]
  P1 --> P3["Phase 3<br/>/api/web"]
  P0 --> W0["kiflet-web bootstrap,<br/>design system"]
  P0 --> M0["kiflet-mobile bootstrap,<br/>SQLite, sync engine"]
  P3 --> P4["Phase 4<br/>pages cut over"]
  W0 --> P4
  P2 --> P5["Phase 5<br/>driver API, app, pilot"]
  M0 --> P5
  P4 --> P6["Phase 6<br/>remove the old"]
  P5 --> P6
```

Rough sizes for one experienced full-time developer (two in parallel
shorten it by about a third, with web and mobile split between them):

| Phase | Size |
|---|---|
| 0: groundwork | 1 week |
| 1: modules in place | 3–4 weeks |
| 2: RabbitMQ, Redis, processes, partitions, load test | 4–5 weeks |
| 3: `/api/web` for every page | 4–6 weeks (overlaps phase 4) |
| 4: kiflet-web, ~45 pages | 8–10 weeks |
| 5: kiflet-mobile and the pilot | 6–8 weeks, plus the pilot's length |
| 6: removal and docs | 1 week |

About **6 to 8 months** for one developer. These are estimates from the
size of the code today, not commitments.

### Against the roadmap

`ROADMAP.md`'s "Toward the first paying customer" does not wait for this.
The provider onboarding, the Amharic review and the pilot with one company
can run on today's app while phases 0–2 happen underneath it; none of
those phases changes what a user sees. The one roadmap item this replaces
is **"Tracking with the screen off"** (a Capacitor wrapper): kiflet-mobile
does that natively. If a pilot needs screen-off tracking before phase 5,
starting phase 5 early (with the current `/api/v1/driver` API) is better
than a Capacitor wrapper that would be thrown away.

---

## 9. Risks

| Risk | What it would cost | What reduces it |
|---|---|---|
| Two dashboards to maintain during phase 4 | features land twice or wait | freeze new dashboard features in Go once phase 4 starts; new features go only to kiflet-web |
| Rules drift into TypeScript | a role or transition behaves differently on web | the API returns allowed actions; review checklist item; no status logic in clients |
| Background GPS policies (Play, Android vendors killing services) | gaps in trails | foreground service with notification; tested on the cheap phones drivers use (Tecno, Itel, Samsung A); gaps drawn honestly on the maps (PR #46) |
| RabbitMQ or Redis misconfigured in production | lost or duplicated messages, slow pages | quorum queues, confirms, idempotent consumers; failure tests in CI; staging identical to production |
| The partition migration of `tracking_events` | a slow or failed copy on a big table | copy by day in the background, swap in one transaction, keep the old table a week |
| Operating more moving parts on VMs | more to monitor and back up | the alerts in the architecture's section 14; runbooks in `docs/deploy.md` for each part |
| Breaking changes to `/api/driver` | old apps on phones fail | version header, `426`, support two releases, tests against the previous app version |
| Cost of accounts and services | Apple's yearly fee, EAS plans, more VMs | start Android-only; EAS's free tier until builds are frequent; three VMs until load needs more |

---

## 10. Still to decide

Questions this plan doesn't settle, for when they come up:

1. **The GitHub organization** `kiflet` before creating the two new
   repositories (phase 0, step 1)?
2. **iOS:** at the same time as Android, or later? Android-only halves the
   store work and the testing; most drivers use Android.
3. **Object storage in production:** SeaweedFS or MinIO on our VMs, or a
   hosted S3 (which stores the backups off the machine, a ROADMAP
   requirement for the pilot)?
4. **The public pages' address:** `track.kiflet.et` or `kiflet.et/track`
   (shorter in SMS)?
5. **Who builds what:** if a web or mobile developer joins, phases 4 and 5
   can run in parallel with phase 2.
