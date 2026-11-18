# Changelog

What changed in each release, newest first.

## Unreleased

### Fixed

- **The overview's numbers on a laptop.** At 1280 pixels wide, the five
  numbers at the top of the overview (and of the platform's health page)
  wrapped onto two rows; they now stay on one, and go three, then two, to
  a row on narrower screens.

### Added

- **Support by phone and Telegram.** `SUPPORT_PHONE`, `SUPPORT_TELEGRAM`
  (a username) and `SUPPORT_HOURS` add those ways to reach support to the
  Help page, beside the email address. Each is left off when not set.

### Changed

- **The driver app in the dashboard's look** (the redesign, part 9). The
  same colours (Desk, and Night in dark mode), the Geist font, bundled so
  it works offline, the dark mark in place of the blue one, and the
  dashboard's status badges, buttons, fields and cards, kept at the
  driver app's larger sizes for one hand and bright sun. The app's icon,
  the browser's colour around it and the mini-map's markers follow.
  Drivers see it the next time the app starts after downloading it.
- **The maps in the theme's colours.** Routes, the truck's area, the start
  and a delivery on the dashboard's and tracking page's maps now use the
  palette (and Night's in dark mode) instead of a fixed blue; so does the
  browser tab's icon.
- **The public pages redesigned** (the redesign, part 8).
  - **Tracking**: the carrier first, then the status, the expected time,
    the distance left by road and where the truck was last seen; when
    each stage was reached; the delivery's details, what happens on
    arrival, and a button to call the carrier beside the map.
  - **Sign in**: a choice of phone or email, which only changes the field;
    links for drivers, new companies and customers below.
  - **Find my shipments**: a box for a tracking number under the phone
    form.
  - On a phone, the public pages' header fits on one line.
- **The company's pages redesigned** (the redesign, part 7).
  - **Help**: answers to common questions by topic, with a search that
    narrows them as you type, then your questions to the Kiflet team and
    how to reach support.
  - **Team**: members, those invited who haven't signed in yet (resend or
    cancel), and former staff, with a search; the invite form and what
    each role can do beside them.
  - **Settings**: in sections with a menu beside them, the customer texts
    as switches, and a preview of the text a customer gets, in the
    language chosen, with how many SMS it costs.
  - **Billing**: the plan's price first, this month's numbers, the
    invoices as a full table, and how to pay and what the invoice says
    beside them.
  - **Reports**: the six numbers in one row, and on-time bars by route.

- **Alerts redesigned, with rules** (the redesign, part 6). A company sets
  when it is alerted: problems reported by drivers, running late (by how
  much), phone offline (for how long), a long stop outside a town (new) and
  leaving the route (by how far), each of which can be turned off. An
  alert is resolved with a note of what was done, and the page lists what
  was resolved today and by whom. An administrator chooses who also gets
  alerts by email or SMS, every alert or problems only. The office's
  "customer reported a problem" alerts now offer to text the customer, as
  they were meant to.
- **Drivers and vehicles redesigned, with their papers** (the redesign,
  part 5). A driver has a licence expiry date, and a vehicle its insurance
  and annual inspection (ቦሎ) dates. The office is alerted 30 days before
  each, and critically once it has passed; recording the new date resolves
  it. A driver whose licence has run out can't be given a trip. The drivers
  page has tabs (on a trip, available, off duty), each driver's current
  load and where their phone was last heard from; the vehicles page has the
  fleet at a glance and "papers due in 30 days". A driver can be texted a
  reminder to renew. The dates are in the company's export. Alerts can now
  be about a vehicle.
- **A shipment's documents and charges** (the redesign, part 4). Two
  tabs on a shipment's page. **Documents** keeps files with a load (the
  signed waybill, customs papers, invoices), up to 10 MB each, PDFs and
  photos only, checked by their content; only the company sees them.
  **Charges** lists what the customer is charged, line by line with a
  total, discounts as negative lines. Both are in the company's export.
  The keeping period removes documents with the customer's other details,
  while charges stay for reports.
- **Booking redesigned, and pieces on a load** (the redesign, part 3).
  The booking form is in sections (customer, route and pickup, load,
  driver and priority) with a summary beside it that follows what you
  type: the distance and time on the road (from your own past trips on
  the route when there are enough), when it would arrive, and how full
  the chosen truck will be. A load can say how many pieces it is, such
  as "152 bales": on the shipment's page, the waybill, the list, both
  exports, the CSV import, the API (`cargo.pieces` and `cargo.unit`)
  and the driver app.
- **Dispatch pages redesigned** (the redesign, part 2). The overview
  opens on the date, the figures with icons, the map, what needs
  attention (with buttons to call the driver or customer) and a shipments
  board in tabs: on the road, needing attention, waiting for a driver,
  delivered. Each load shows how far along its route it is. The shipment
  list has counts on its tabs, search and filters in a toolbar, a
  progress column and a menu on each row. A shipment's page has a strip
  of the stages it has passed, the distance left, its speed and where it
  was last seen, with its history and messages in an Activity tab, and
  the driver, customer and load beside it.
- **A new look for the dashboard** (the redesign, part 1). A dark side
  menu with the company at the top and your account at the bottom, a top
  bar with where you are, search (press `/`) and alerts, and a footer on
  every page. Buttons, fields, tables, tabs and status badges follow the
  new design throughout, in the Geist typeface (served from Kiflet itself),
  and dark mode has its own palette. The pages themselves are redesigned
  in the parts that follow.
- **Restoring to the minute** (the rest of audit O1). With
  `WAL_ARCHIVE=on`, Postgres archives every change; `scripts/ship-wal.sh`,
  from cron, encrypts it and copies it to the backup bucket, `backup.sh`
  adds a base backup, and `scripts/restore-pitr.sh` restores the database
  to any moment of the last two weeks. `fetch-backup.sh` brings the base
  and WAL back on a new machine. Staging archives, and the end-to-end
  tests restore to a moment between two sign-ups.
- **The app can run as two instances** (audit O4). `APP_REPLICAS=2`
  starts two behind Caddy, which keeps each client address on one of them
  so that the rate limits hold, and tries the other when one is down. Live
  updates now pass through Postgres, so a dashboard sees a change whichever
  instance made it, and instances starting together take turns at the
  migrations. Staging runs two. With one, nothing changes.
- **Dependencies updated**: the Go modules Dependabot proposed (pgx 5.13,
  chi 5.5, httprate 0.16, goose 3.30 and others), Alpine 3.24, and
  Prometheus 3.16, Grafana 13.3, rclone 1.76, Mailpit 1.33 and
  SeaweedFS 4.53 in the deployments.
- **The live map reads one row a driver** (audit O3). Each driver's
  newest position is kept in its own table as positions arrive, where
  the map used to search a week of them on every load. The migration
  fills it from the last week. A phone's saved positions now go to the
  database a thousand to a statement, not one by one. Nothing looks
  different.
- **The driver app is in modules.** Its 1,686 lines of `app.js` are now 19
  ES modules: the state, the text in both languages, IndexedDB, the
  server, the outbox and sync, settings, GPS, the router, the signature
  box, and one module per screen. They run as written, with no build
  step, and `index.html` preloads them all so start-up doesn't wait on
  one import after another. Nothing changes for drivers.
- **The driver app updates as a whole.** Its service worker used to fetch
  each file afresh whenever there was signal, so an update cut short on a
  weak signal could leave old and new files mixed. Now the server lists
  every file of the app with its SHA-256 in `/driver/build.json`. When the
  app starts, and hourly while it runs, the worker downloads a new version
  beside the old one and keeps it only once every file matches. The new
  version takes over the next time the app starts, or sooner when the
  driver comes back to the app with no form open. The app now starts from
  the cache at once, signal or not, where before it waited up to three
  seconds for the network.

### Added

- **Webhooks** (FR-047). An admin adds an address under **Settings →
  Webhooks** and chooses what it is told: a shipment booked, a status
  changed, a delivery, or a problem reported by the driver or the
  receiver. Each message is the shipment as the API sends it and what
  happened, signed as the Standard Webhooks specification describes, so
  any of its libraries can check it. A receiver that doesn't answer 2xx is
  tried again six times over about a day. Each webhook's page lists the
  last 30 days of messages with their answers, and can send a test, send
  one again, pause, take a new secret or remove the webhook. In
  production a webhook must be HTTPS on a public address, checked again
  as each message connects (`WEBHOOK_PRIVATE_TARGETS` allows others, for
  development).
- **A public API** (FR-046). A company's own systems can book, list,
  change, assign and cancel its shipments, read each one's history and
  where its truck was last seen, and list its drivers, in JSON at
  `/api/v1`. An admin makes keys under **Settings → API keys**, named after
  what uses them; a key shows once, only its SHA-256 is kept, and revoking
  it stops it at once. A key acts for the whole company under the
  dashboard's rules, texts included, and the shipment's history and the
  audit log record what it did under its name. Each key may make 300
  requests a minute. `/api/v1/openapi.json` describes every endpoint and
  field, and a test keeps it true to the code. Every plan has it.
- **Shipments can be booked from a CSV file** (FR-013). **Import CSV** on
  the shipment list takes a spreadsheet saved as CSV, up to 200
  shipments. Every row is checked as the booking form would check it and
  shown with what is wrong, before anything is booked; a file with a row
  that can't be booked books nothing, so it can be corrected and sent
  again whole. The page lists the columns and offers a template, and the
  list's own export can be edited and sent back: columns the import
  doesn't know are ignored. Commas or semicolons both work. Each
  shipment is booked as the form books it, texts included, and the audit
  log records the import. Drivers are assigned afterwards from the list.
- **A shipment can name its receiver** (audit W4). The booking form has
  a second, optional person: who takes delivery at the destination, when
  that isn't the customer who booked. Both get the booked, departed,
  delayed and cancelled texts. What belongs to the handover goes to the
  receiver alone: the arrival text, the driver's Call button and
  suggested names in the driver app, and the link to confirm receipt.
  The customer is told the delivery was made, with the tracking link.
  Either finds the shipment with their phone on `/track/phone`. The
  shipment page, the search, both exports and the waybill (Shipper and
  Consignee) carry the receiver, and the keeping period removes them
  with the customer. A shipment without one works as before: the
  customer receives. Texts double for a shipment with two people.
- **Customers can be written to in Amharic** (audit W1, the customers'
  half). Settings has a language for customers, English or Amharic. A
  company that chooses Amharic sends its customers Amharic texts, and
  their tracking and confirmation pages open in Amharic. Every customer
  page has a switch to the other language, and remembers the choice; the
  pages that belong to no company (`/track`, finding shipments by phone)
  follow the choice, then the browser's language. Dates stay in the
  Gregorian calendar, with the months spelled out. A text in Amharic
  holds 70 characters to a part, so most go as two or three parts of the
  allowance, and Settings says so. The words are in `internal/text`, one
  file to a language. **The Amharic was written without a native
  reviewer and needs one** before a company turns it on. The dashboard,
  the mail and the drivers' texts are still English only.
- **Backups leave the machine, encrypted** (audit O1). With `backup.env`
  (`backup.env.example` is the template), `scripts/backup.sh` encrypts each
  backup with age, to public keys whose private keys are kept off the
  server, and copies it to an S3-compatible bucket, which keeps it 90 days.
  It won't copy a backup that isn't encrypted. `scripts/fetch-backup.sh`
  brings one back from the bucket with neither `.env` nor a running stack,
  and `scripts/restore.sh` restores an encrypted one with `AGE_IDENTITY`,
  decrypting it before anything is replaced. `docs/deploy.md` has the
  set-up, restoring on a new machine, and the drill, which `make test-e2e`
  runs on staging. It now needs age.
- **A backup holds the machine's settings too**: `.env`, `backup.env` and
  `deployments/secrets/`, in a third file, `kiflet_<time>_settings.tar.gz`,
  so a new machine can be set up from the backup alone.

### Fixed

- **A change made during a sync waited for the next one.** What the
  driver recorded while a sync was under way, a status, a message or a
  delivery, stayed on the phone until the next regular sync, a minute
  later or five with battery saver. On a weak signal, where a sync with
  photos can take a minute, that would be often. It now goes as soon as
  the sync under way ends.
- **The signature box could drop the first stroke.** It measured itself
  on the first frame after the delivery form opened, and ignored a stroke
  begun before then. It now measures itself on the first touch if the
  frame hasn't come yet.

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
