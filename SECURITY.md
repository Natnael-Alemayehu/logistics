# Security

## Reporting a vulnerability

Report it privately, not in a public issue. On GitHub, open this
repository's **Security** tab and choose **Report a vulnerability**.

Say what you did, what happened and what you expected, with the request or
the steps if you have them. We try to answer within a week. There is no bug
bounty.

Only the newest release, and `main`, get fixes.

## Where to look

Kiflet holds several companies' shipments, drivers and customers in one
database. The most serious finding is one company, or someone outside,
reading or changing another company's data. Every query is scoped by
company, and the integration tests try to cross that line.

The surfaces that face the internet:

| Surface | What protects it |
|---|---|
| Sign-in: `/login`, `/driver/` (phone and PIN), `/signup`, `/forgot`, `/reset/phone` | Five wrong passwords or PINs lock an account for 15 minutes. One address gets 30 sign-in attempts a minute, and 10 a minute for anything that sends a code. Answers don't reveal whether an email or phone number is registered. |
| Sessions | A session cookie, revoked on sign-out, password reset, deactivation and suspension. The live feed (a WebSocket) is closed within 30 seconds of any of those. |
| Public tracking: `/track`, and by phone with a texted code | A tracking number shows one shipment's status, never the company's other loads. Lookups are limited to 60 a minute for each address. |
| SMS webhooks: `/api/v1/sms/inbound`, `/api/v1/sms/status` | A shared secret (`SMS_INBOUND_SECRET`). A repeated message is handled once, and replies to one number are limited. |
| Payment notifications: `/api/v1/payments/telebirr/notify`, `/api/v1/payments/cbebirr/…` | telebirr's signature; Basic authentication for CBE Birr. An invoice is settled once however often a notification is repeated. |
| Uploads: delivery photos and signatures | Images are decoded and their size checked before anything is stored. SVG is refused. |
| `/metrics` | A token (`METRICS_TOKEN`), and the proxy refuses the path from outside. |
| Pages | A Content Security Policy with `script-src 'self'` and no inline scripts, no framing, HSTS. |

Texts and codes cost money, so anything that makes the product send them
without limit is in scope too.

## Out of scope

- The provider sandbox (`cmd/sandbox`, `internal/sandbox`). It stands in for
  Ethio telecom, telebirr and CBE Birr in development and tests, and is
  never deployed with real data.
- The demo logins in `README.md`. They exist only in a database loaded with
  `make seed`.
- Flooding the service with traffic.
