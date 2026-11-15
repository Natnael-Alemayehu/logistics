# Deploying Kiflet

One machine with Docker runs it: the app, PostGIS, and Caddy, which gets a
TLS certificate for `DOMAIN` and serves HTTP/2 and HTTP/3.

```bash
cd backend
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
scripts/backup.sh                                   # from backend/
scripts/restore.sh <kiflet_….dump> [<kiflet_…_files.tar.gz>]
```

`backup.sh` writes the database (`pg_dump`) and the app's volume (photos and
signatures kept on disk, and the app secret that sessions and codes depend
on) to `/var/backups/kiflet` (`BACKUP_DIR`), readable by their owner only,
and deletes those older than two weeks (`KEEP_DAYS`). Run it nightly and copy
the backups off the machine:

```
15 2 * * *  cd /srv/kiflet/backend && scripts/backup.sh >>/var/log/kiflet-backup.log 2>&1
```

Photos kept in S3 aren't included: use the store's versioning or
replication. `restore.sh` asks for the stack's name, then stops the app,
replaces the database (and the files, if given) with the backup's, and starts
it again. `make test-e2e` backs up staging and restores it, so both scripts
are tested.

## Staging

Production's compose file and images, with `deployments/staging/compose.yml`
standing in for the outside world: the provider sandbox for Ethio telecom,
telebirr and CBE Birr (the app runs its real adapters, and the sandbox calls
back over HTTPS through Caddy), Mailpit for SMTP, SeaweedFS for S3, and
Caddy's own certificate authority. Prometheus and Grafana are on.

```bash
make staging-up     # from backend/: build, start, and copy Caddy's root certificate to tmp/staging-ca.crt
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
