# Changelog

What changed in each release, newest first.

## Unreleased

### Added

- **A licence.** `LICENSE` says all rights are reserved: the code may be
  read, and nothing else, without written permission.
- `SECURITY.md`, on reporting a vulnerability and where to look for one,
  and `CONTRIBUTING.md`, on running, testing and changing the code.
- `docs/deploy.md`: production, monitoring, backups, staging and connecting
  the providers, moved out of the README.

### Changed

- **Everything moved out of `backend/`** to the repository root, which is
  now where `go.mod`, the `Makefile` and `.env` live and where every command
  runs. The development compose file and the sandbox's settings are in
  `deployments/dev/`; `make db`, `make sandbox` and `make run-sandbox` find
  them.
- The long documents moved to `docs/`: the requirements, the background,
  the manual test guide and the audit, which is now
  `docs/audit-2026-11.md`.

### Fixed

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
