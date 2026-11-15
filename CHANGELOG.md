# Changelog

What changed in each release, newest first.

## Unreleased

Nothing yet.

## 0.9.0 (2026-11-15)

The first tagged release. Kiflet isn't live yet: `1.0.0` comes with the
first paying customer.

**History was rewritten before the tag.** Every commit now carries the
right author name and a no-reply address, and the co-author lines are
gone. The files are exactly as they were, but every commit has a new hash,
so a clone made earlier has to be made again. The audit quotes the new
hashes.

### Added

- **A licence.** `LICENSE` says all rights are reserved: the code may be
  read, and nothing else, without written permission.
- `SECURITY.md`, on reporting a vulnerability and where to look for one,
  and `CONTRIBUTING.md`, on running, testing and changing the code.
- `docs/deploy.md`: production, monitoring, backups, staging and connecting
  the providers, moved out of the README.
- **The driver app is tested in a browser.** An end-to-end test runs it in
  headless Chrome against staging: signing in, starting a trip, and with
  the signal gone, a delivery form that survives a reload and a delivery
  sent once the signal is back. `make test-e2e` needs Chrome or Chromium
  for it, and skips it without one, except in CI.
- **More checks in CI.** `make lint` runs golangci-lint (errcheck,
  ineffassign, staticcheck and unused), parses every script with Node, and
  scans the whole history for secrets with gitleaks; CI runs it on every
  pull request. The integration job adds to each run's summary how much of
  `internal/service` and `internal/web` its tests cover. Dependabot
  proposes updates each week for the Go modules, the Actions and the images.

### Changed

- **One migration instead of 21.** `migrations/20261115000100_baseline.up.sql`
  holds the whole schema, and `git log -- migrations/` shows the migrations
  that built it. A database they built keeps its data and takes the
  baseline as done, since every statement in it can run again; a new one
  gets the same schema, line for line.
- **Everything moved out of `backend/`** to the repository root, which is
  now where `go.mod`, the `Makefile` and `.env` live and where every command
  runs. The development compose file and the sandbox's settings are in
  `deployments/dev/`; `make db`, `make sandbox` and `make run-sandbox` find
  them.
- The long documents moved to `docs/`: the requirements, the background,
  the manual test guide and the audit, which is now
  `docs/audit-2026-11.md`.

### Removed

- **The Twilio SMS adapter**, with `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`
  and `TWILIO_FROM_NUMBER`. Nothing used it and the provider sandbox never
  covered it. `SMS_PROVIDER` is `mock` or `ethiotelecom`; with `twilio` the
  app now refuses to start.
- Code nothing called: the live feed's per-driver and per-shipment rooms
  and its message types (the dashboard only ever had one feed for each
  company), and a handful of unused helpers.

### Fixed

- **A photo stored on disk is only kept whole.** Local storage ignored the
  error from closing a file it had just written, which is where a full disk
  can show up, and could keep half a photo. It now reports the error and
  removes the file.
- **Staging no longer hangs on start.** The job that makes the stand-in S3
  bucket could wait for ever on one call, holding up the app and CI. Each
  call now has five seconds, and `make staging-up` gives up after ten
  minutes.
- `TestRateLimits` no longer fails on a busy machine. When its requests run
  into a new clock minute, which resets the limiter's count, it runs again
  from the start of the next one.
- **Maps work in every build.** The map library (Leaflet 1.9.4) had never
  been committed: an ignore rule meant for Go's `vendor/` directory also
  matched `static/vendor/`. It was served on the machine the product was
  developed on and returned a 404 everywhere else, including CI, staging
  and any production image. It is in the repository now, a unit test fails
  when a page, script or stylesheet names a static file that isn't
  embedded, and the end-to-end tests request the library from staging.

### Renamed

The product is now called **Kiflet** (ቅፍለት, a caravan). Until this release
it was called Menged, a name another transport app in Ethiopia already uses.

Nothing behaves differently, but the name was in many identifiers, so an
installation made before the rename has things to move:

- **Compose projects** are `kiflet`, `kiflet-dev` and `kiflet-staging`.
  Compose names volumes after the project, so the new project starts with
  empty ones. Move a server's data with `scripts/backup.sh` under the old
  project and `scripts/restore.sh` under the new one; don't start the new
  project on top of it.
- **A deployment's `.env`** keeps what it says. Change `APP_NAME`,
  `MAIL_FROM`, `SUPPORT_EMAIL`, `ETHIO_TELECOM_SENDER_ID` and anything else
  there that carries the old name; `.env.example` shows the new values.
- **Backups** default to `/var/backups/kiflet` and are named
  `kiflet_<time>.dump`. `scripts/backup.sh` only prunes files with that
  prefix, so older backups stay until removed by hand.
- **The database** setting that permits purging the append-only tables is
  `kiflet.purge`. A migration recreates the trigger function for databases
  made before the rename.
- **Metrics** are `kiflet_*`. The Grafana dashboard and alert rules use the
  new names; history recorded under the old ones is not carried over.
- **The driver app** keeps its unsent work in a database whose name
  changed. Sync every phone before updating, or that work is lost.
- **In the browser**, a dispatcher's hidden columns and notification
  setting reset once.
- **The Go module** is `github.com/natnael-alemayehu/kiflet`, and the
  binaries are `kiflet` and `kiflet-sandbox`.
- **Demo logins** are at `kiflet.et`, with the password `kiflet-demo`.
