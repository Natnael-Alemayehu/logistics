# Deploying Kiflet

One machine with Docker runs it: the app, PostGIS, and Caddy, which gets a
TLS certificate for `DOMAIN` and serves HTTP/2 and HTTP/3.

```bash
cp .env.example .env    # set DOMAIN, DB_PASSWORD, SMS_* and SMTP_* settings, BILLING_BANK_DETAILS
docker compose -f deployments/docker-compose.prod.yml --env-file .env up -d --build --wait
```

In production the app refuses to start without an `https://` address (the
compose file sets `PUBLIC_URL` from `DOMAIN`), with a stand-in provider or
`SANDBOX_URL` unless `ALLOW_MOCK_PROVIDERS=true`, or with a provider whose
settings or key files are missing, and says which. The database password is
passed to the app on its own, not in a URL, so it may contain any character.
Provider keys, such as telebirr's key files, go in `deployments/secrets/`,
which the app sees at `/run/secrets` (see the README there). Signatures and
photos go to the `app-data` volume, or to S3 or another S3-compatible store
with `STORAGE_TYPE=s3`.

Each container has a read-only root filesystem where it can, no Linux
capabilities beyond what it needs and no privilege escalation, and its logs
rotate. When the app is stopped or replaced, it stops taking requests and
gives those in progress, and the text or email being sent, 30 seconds to
finish. Messages it hadn't started are left for the next start to send at
once.

Run one app container per database. Live updates (the WebSocket hub) and
rate limits are kept in memory, so a second instance would split them. The
background jobs take a Postgres advisory lock, so they never run twice.

Add `--profile monitoring` for Prometheus and Grafana: set `METRICS_TOKEN`, and
put the same value in `deployments/prometheus/metrics_token`. Grafana listens
on `127.0.0.1:3000` (reach it over an SSH tunnel) and comes with a **Kiflet**
dashboard: requests, errors and response times by route, driver sync, texts
and emails sent and waiting, background jobs, and memory. `/metrics` labels
requests by route pattern (`/app/shipments/{id}`), never by company or ID.

The maps' satellite view uses Esri World Imagery, which needs an ArcGIS
licence for commercial use. Point `MAP_SATELLITE_URL` at your own imagery
provider, or set it to `off`.

Point your SMS provider's webhooks at `https://DOMAIN/api/v1/sms/inbound`
(texts from phones) and `https://DOMAIN/api/v1/sms/status` (delivery
reports), with `SMS_INBOUND_SECRET` in an `X-Webhook-Secret` header or a
`secret` parameter.

Health checks: `/healthz` (process up) and `/readyz` (database reachable).

## Backups

```bash
scripts/backup.sh
scripts/fetch-backup.sh [<time>]
scripts/restore.sh <kiflet_….dump[.age]> [<kiflet_…_files.tar.gz[.age]>]
```

`backup.sh` writes three files to `/var/backups/kiflet` (`BACKUP_DIR`),
readable by their owner only:

- the database (`pg_dump`);
- the app's volume: photos and signatures kept on disk, and the app secret
  that sessions and codes depend on;
- the machine's settings: `.env`, `backup.env` and `deployments/secrets/`.

It deletes those older than two weeks (`KEEP_DAYS`). Run it nightly:

```
MAILTO=you@example.et
15 2 * * *  cd /srv/kiflet && scripts/backup.sh >>/var/log/kiflet-backup.log 2>&1
```

It exits with an error when any step fails, the copy off the machine
included, and cron emails `MAILTO` when it does. Photos kept in S3 aren't
included: use the store's versioning or replication.

### Off the machine, encrypted

A backup on the machine it protects is lost with it. With `backup.env`,
each backup is encrypted with [age](https://age-encryption.org) as it is
written and copied to an S3-compatible bucket:

1. Install age on the server: `apt install age`.
2. On your own computer, not the server, make a key pair with
   `age-keygen -o kiflet-backup.key`. It prints the public key (`age1…`).
   Keep the private key where a lost server can't take it: in a password
   manager, and on a USB stick in a safe. Make a second pair for a second
   person, so that one lost key loses nothing.
3. Make a private bucket with a provider other than the server's (Backblaze
   B2, Cloudflare R2, Wasabi and AWS all work), with versioning or object
   lock if it offers them, and keys that can list, read and write it.
4. Copy `backup.env.example` to `backup.env`, beside `.env`, and fill in the
   public keys, the bucket and its keys. It is kept out of `.env` so the app
   never sees them.

Nothing leaves the machine unencrypted: with a bucket and no key,
`backup.sh` stops before it starts. The server holds only the public keys,
so neither the bucket's provider nor anyone with the bucket's keys can read
a backup. Without a private key nobody can, you included: keep the keys as
carefully as the data. The bucket keeps backups 90 days
(`BACKUP_S3_KEEP_DAYS`; 0 leaves it to the bucket's own rules). The copying
is rclone in a container (`deployments/docker-compose.backup.yml`), so the
server needs only Docker and age.

### Restoring

`restore.sh` asks for the stack's name, then stops the app, replaces the
database (and the files, if given) with the backup's, and starts it again.
An encrypted backup needs `AGE_IDENTITY`, the private key's file. Each file
is decrypted first, so the wrong key or a damaged copy changes nothing.

On a new machine, the old one gone:

1. Install Docker and age, clone the repository at the release that was
   running (or a newer one: the app brings an older database up to date),
   and copy a private key onto the machine.
2. Write `backup.env` with the bucket's settings, and fetch the newest
   backup with `scripts/fetch-backup.sh`. `scripts/fetch-backup.sh
   20261104_021500` fetches an older one. It doesn't need `.env`, which is
   in the backup.
3. Unpack the settings into the checkout:
   `age -d -i kiflet-backup.key /var/backups/kiflet/kiflet_<time>_settings.tar.gz.age | tar -xzf -`
4. Start the stack with the `docker compose` command at the top of this
   page, then restore:
   `AGE_IDENTITY=kiflet-backup.key scripts/restore.sh` with the `.dump.age`
   and `_files.tar.gz.age` files that `fetch-backup.sh` printed.
5. Point the domain at the new machine, and delete the private key from it.

### The drill

A backup nobody has restored may not restore. Once a quarter, and whenever
`backup.env` changes, follow the steps above on a spare machine, with a
private key from where it is kept, and check that the newest data is there.
`make test-e2e` runs the same round trip on staging for every change: an
encrypted backup copied to a bucket, the copies on the machine lost, and
the backup fetched and restored.

## Staging

Production's compose file and images, with `deployments/staging/compose.yml`
standing in for the outside world: the provider sandbox for Ethio telecom,
telebirr and CBE Birr (the app runs its real adapters, and the sandbox calls
back over HTTPS through Caddy), Mailpit for SMTP, SeaweedFS for S3, and
Caddy's own certificate authority. Prometheus and Grafana are on.

```bash
make staging-up     # build, start, and copy Caddy's root certificate to tmp/staging-ca.crt
make test-e2e       # the end-to-end tests, against staging
make staging-down   # remove staging and its data
```

| | |
|---|---|
| The app | https://kiflet.localhost:8443 |
| The sandbox: phones, wallets and faults | https://sandbox.kiflet.localhost:8443 |
| Email sent (Mailpit) | https://mail.kiflet.localhost:8443 |
| Grafana, as `admin` / `staging-grafana-password` | https://grafana.kiflet.localhost:8443 |

It listens on 127.0.0.1 only, and `*.localhost` names reach this machine
without DNS. Trust `tmp/staging-ca.crt` in the browser, or give it to curl
with `--cacert`. `make staging-logs` follows the logs.

The end-to-end tests (`tests/e2e/`) reach staging as browsers and the
providers do. A company signs up with the code from the sandbox's phone and
the link from Mailpit. A driver works a load in the driver app, with its
photo stored in S3 and the dashboard told over a WebSocket. Invoices are
paid with telebirr and CBE Birr. The tests also check the HTTPS edge (the
redirect, the headers, and that a client can't claim another address) and
the figures in Prometheus and Grafana, and last, take a backup and restore
it.

One test runs the driver app itself in headless Chrome, the size of a
phone: signing in and choosing a PIN, starting a trip, then, with the
signal gone, a delivery form that survives the app being loaded again and
a delivery that is sent once the signal is back. It finds Chrome or
Chromium on the `PATH`, or the program named in `E2E_CHROME`, and is
skipped without one, except in CI.

## Connecting the real providers

The adapters follow what each provider publishes, and the sandbox plays the
same contract. What to settle when the credentials arrive:

| Provider | Contract | Settle at onboarding |
|---|---|---|
| telebirr | H5 C2B web checkout as published: token, preOrder, checkout, notify, queryOrder | the signature (RSA-PSS with SHA-256 is assumed, as in the maintained SDKs), the `order_status` values queryOrder returns (one table in `pkg/payments/telebirr.go`), and whether notifications are retried |
| CBE Birr | none published: pay by reference, with query, validation and confirmation callbacks behind Basic authentication, as in `pkg/payments/cbebirr.go` | the field names, result codes and login, or adapt that file to CBE's |
| Ethio telecom | none published: token, send, delivery reports and inbound texts, as in `pkg/sms/ethiotelecom.go` | the whole contract |

Then:

1. Set the providers' settings (see `.env.example`): telebirr's API and
   checkout URLs, app and merchant IDs, app secret and the two key files;
   CBE Birr's merchant ID and callback login; Ethio telecom's API URL, key
   and sender ID. Unset `SANDBOX_URL`.
2. Give the providers the app's addresses: telebirr is sent
   `https://DOMAIN/api/v1/payments/telebirr/notify` with each order; CBE
   Birr calls `https://DOMAIN/api/v1/payments/cbebirr/query`, `/validation`
   and `/confirmation`; Ethio telecom posts to the SMS webhooks above.
3. Where a provider's contract differs from ours, change the adapter and the
   sandbox's side (`internal/sandbox/sms.go`, `telebirr.go`, `cbebirr.go`)
   together, and run `make test-integration`: its sandbox tests run the real
   adapters against the sandbox.
4. Pay a small invoice with each provider, and send a text to a driver's
   phone, before opening sign-ups.
