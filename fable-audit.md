# Kiflet audit

Usability and logic audit of the whole product: the dispatcher dashboard and
public pages (web), the Go backend, and the driver app (mobile).

- **Date:** 2026-11-12
- **Commit audited:** `d0d6204` on `product-consolidation`, now on `main`
- **Auditor:** Claude Fable 5.2 (Claude Code)
- **Paths** are relative to the repository root. In the tables they are
  shortened: `service/`, `model/`, `static/` and `templates/` sit under
  `backend/internal/` (the last two under `backend/internal/web/`), and
  `sqlc/queries/` under `backend/`. Line numbers are as of the commit above
  and will drift.

## Verdict

The codebase is structurally sound, but it is not ready for paying customers
yet. Six real bugs were reproduced, one abuse hole lets a throwaway signup
send SMS at the operator's cost, and the driver app is a PWA that only
records GPS while it is open, which undercuts the live-tracking promise.

## How to use this document

Every finding has an ID (`R1`, `S2`, `M3`, ...) so that commits, issues and
code comments can refer to it. Each one is marked with how it was established:

- **Reproduced**: observed against a real PostGIS database with a temporary
  integration test (since deleted; the steps are written out below).
- **Read**: found by reading the code. Not executed. Treat as likely, and
  confirm before relying on it.

When a finding is fixed, add the commit hash next to it instead of deleting
it, so this stays a record of what was found.

## Fix record

Fixed on `product-consolidation` on 2026-11-14, one commit per group of
findings. Each finding below carries its commit.

| Commit | Findings |
|---|---|
| `56a3133` | R1, R5, S5, L10, W6, W7 |
| `de63535` | S1, S2, L5, W8 |
| `2b3fc83` | R2, R3, R4, L8, L9 |
| `45c9578` | R6, S4 |
| `c6b2fa8` | M2 to M8 |
| `0a270db` | W2, W3, W9 |
| `36446b6` | L1, L2, L3, L4, L6, L7, S6 |
| `103a129` | S3 |
| `3654d0e`, `b6cfd41` | R7, O2, O5 (the alert rules were corrected on staging in `b6cfd41`) |
| `05611d9` | M1 (wording), W5 |

Still open:

- **W1** (Amharic on the dashboard, tracking pages and customer texts) needs
  a native speaker. The new driver-app strings from `2b3fc83` and `c6b2fa8`
  were drafted in Amharic and need the same review.
- **W4** (separate shipper and consignee) is a schema change.
- **M1** is resolved in wording only. Tracking with the screen off needs a
  native wrapper with background location.
- **W7**: the error is explained, but a phone number still belongs to one
  account across all companies.
- **O1, O3, O4, O6**: infrastructure and provider contracts, not code
  changes.

Departures from the requirements that these fixes made (the text limits,
cancelling a subscription, tracking while the app is open) are recorded in
Appendix D of `functional_requirements.md`.

## Scope and method

### What was read

- All of `backend/internal/service` (business rules), including all of
  `billing.go`.
- The HTTP layer: `backend/internal/web/server.go`, `middleware.go`,
  `render.go`, `public.go`, `api_driver.go`, `app_dashboard.go`.
- All of `backend/sqlc/queries` and `backend/migrations`.
- `backend/internal/model/status.go` and `plan.go`.
- The dashboard's `static/app.js`, all of the driver app
  (`static/driver/app.js`, `sw.js`, `index.html`, `manifest.webmanifest`).
- Templates: both layouts, `app/shipment_form.html`, `app/shipment.html`.
  Other templates were only checked by loading the pages.
- `backend/pkg/storage/pod.go`, `backend/cmd/server/main.go`.
- Deployment: `docker-compose.prod.yml`, `Caddyfile`, `Dockerfile`,
  `scripts/backup.sh`, the Grafana provisioning directory.

### What was run

| Check | Result |
|---|---|
| `go vet ./...` | Clean |
| Unit tests (`make test`) | All pass |
| Integration tests (`go test -tags integration ./tests/integration/`) | All pass except `TestGatewayFaults` (see R7) |
| Temporary probe test against PostGIS | Reproduced R1 to R6 |
| Demo server, signed in as the demo admin | The 13 main `/app/*` pages returned 200 in 2 to 16 ms, each under 35 KB |
| Headless Chrome screenshots at 390 px | Landing and public tracking pages render correctly |

### What was not checked

- **No interactive click-through.** The Chrome extension was not connected,
  so no forms were submitted through a browser and no page was driven by
  hand. Dashboard usability findings come from the templates, the JS and the
  page loads above.
- **The driver app was never seen rendered.** The one headless screenshot
  caught it at "Loading…" before it booted. Every driver-app finding is from
  reading the code.
- **The staging end-to-end suite** (`make staging-up && make test-e2e`) was
  not run.
- **Real provider contracts** (Ethio telecom, telebirr, CBE Birr) cannot be
  audited from code. Only the sandbox's version of them was exercised.
- **No real device testing**, in particular no low-end Android phone.

## Reproduced bugs

### R1. Account lockout never resets (High)

**Fixed in `56a3133`.** Locking clears the counter.

- **Where:** `backend/internal/service/auth.go:138-157`;
  `IncrementFailedLoginAttempts` and `LockUserAccount` in
  `backend/sqlc/queries/users.sql`.
- **Observed:** five wrong PINs return 403 (locked), as intended. Once the
  15-minute lock has expired, a single mistyped PIN locks the account again
  at once, and the correct PIN sent right after is also refused.
  `failed_login_attempts` was 6 at that point.
- **Cause:** the counter is only cleared by a successful login. An expired
  lock leaves it at or above the limit of 5, so the next failure re-locks.
- **Impact:** a driver who mistypes once after a lockout is locked out
  again. Anyone who knows a phone number or email can keep that account
  locked with one request every 15 minutes.
- **Fix:** set the counter to zero when an expired lock is found (or at the
  moment of locking).

### R2. A slow phone clock makes status updates "stale" (Medium)

**Fixed in `2b3fc83`.** Every offset is applied, measured when the request arrived.

- **Where:** `backend/internal/service/sync.go:228-243` (offset only applied
  when it exceeds the 2-minute `clockTolerance`);
  `backend/internal/service/shipment_status.go:87-106` (stale check against
  `status_changed_at`).
- **Observed:** a phone whose clock is 90 seconds slow reports "Start trip"
  just after the office assigned the load. The update comes back `STALE` and
  the shipment stays `assigned`.
- **Cause:** skew under two minutes is left uncorrected, but the staleness
  comparison is exact to the second.
- **Impact:** real updates are dropped for drivers whose phone clocks are a
  little behind, which is common on phones without network time.
- **Fix:** always apply the measured offset, or give the stale check a
  tolerance of a few seconds rather than none.

### R3. A resent sync reports a false conflict (Low to Medium)

**Fixed in `2b3fc83`.** A change seen before by its client ID gets its first answer again.

- **Where:** `applyStatus` in
  `backend/internal/service/shipment_status.go:87`.
- **Observed:** the phone sends two status updates (depart, then arrive) and
  both are saved. When the identical request is sent again, as happens when
  the first response is lost, the first update comes back `STALE` ("shipment
  was changed while you were offline") and the second comes back OK.
- **Impact:** the data is correct and no alert is raised, but the driver sees
  a "Not applied" notice for something that was applied.
- **Fix:** look up an existing event with the same client ID before the
  stale check, and answer a replay with success.

### R4. Drivers receive their own messages, and blank ones (Low)

**Fixed in `2b3fc83`.**

- **Where:** `ListDriverMessagesSince` in
  `backend/sqlc/queries/shipment_events.sql`;
  `backend/internal/service/shipment_status.go:327-330`;
  `backend/internal/web/static/driver/app.js:563-577`.
- **Observed:** a note the driver sends comes back in the next sync's
  `messages` with the driver's own name as sender. The app lists it under
  "From the office" with a toast and vibration. Cancelling a shipment without
  typing a note delivers a message with empty text.
- **Cause:** the query returns every event with `driver_id = driver AND
  visible_to_driver`, including those the driver authored. The cancel event
  is marked visible to the driver even when its note is empty.
- **Fix:** exclude events where `actor_id` is the driver; give cancel and
  reopen events a default text.

### R5. Password reset links go to unconfirmed email addresses (High)

**Fixed in `56a3133`.**

- **Where:** `RequestPasswordReset` in
  `backend/internal/service/accounts.go`.
- **Observed:** a staff account (admin role) created with an email that was
  never confirmed. A request to `/forgot` sent "Reset your Kiflet password"
  to that address.
- **Impact:** an admin who mistypes a new colleague's address hands that
  account, with its role, to whoever owns the mistyped address.
- **Fix:** send reset links only when `email_verified_at` is set. An admin
  can still reset the password by hand.

### R6. Uploaded photos have no pixel limit (High)

**Fixed in `45c9578`.** Images are measured from their header and refused over 4096 px a side.

- **Where:** `decodeBase64Image` in `backend/pkg/storage/pod.go:170-190`.
- **Observed:** a PNG of 8000×8000 pixels, 565 KB as sent, was accepted and
  cost 69 MB of allocation to decode.
- **Cause:** `image.Decode` is called with no check on dimensions. Only the
  encoded size is limited (4 MB per photo), and a highly compressible image
  of that size can decode to gigabytes.
- **Impact:** any driver account, including one on a free-trial company, can
  exhaust the memory of the one server that every company shares.
- **Fix:** call `image.DecodeConfig` first and reject anything over a pixel
  budget (about 4000×4000 is generous for a phone photo).

### R7. `TestGatewayFaults` fails (Low; test only)

**Fixed in `3654d0e`.** The test counts only texts sent after barring.

- **Where:** `backend/tests/integration/sandbox_test.go:404-442`, failing at
  line 440 with "a barred number received 1 texts".
- **Observed:** failed 3 of 3 runs in isolation.
- **Cause:** a race in the test. The driver's welcome SMS is delivered
  before the test bars the number. The product behaves correctly.
- **Impact:** CI is likely red or flaky.
- **Fix:** bar the number before creating the driver, or count only texts
  sent after barring.

## Security and abuse (read)

### S1. SMS relay at the operator's cost (High)

**Fixed in `de63535`.** Trial cap, hourly ceiling per company, and `OTP_DAILY_BUDGET` with an alert at 80% (alert rule in `3654d0e`).

Several rules combine into one hole:

- The company name is free text (up to 120 characters) and leads every SMS.
- Any phone number can be added as a "driver" and sent free-text notes and
  broadcasts. Booking a shipment texts any customer number.
- Trial companies are never blocked when they pass their text allowance and
  are not billed for it.
- Sign-up codes are sent to any Ethiopian number: 3 an hour per number and
  10 a minute per IP, with no overall budget.

A throwaway signup can therefore send spam or phishing under the operator's
sender ID, and a script can run up the SMS bill by requesting codes.

**Fix:** a hard cap on trial texts, a per-company hourly throttle, and a
global daily budget for verification codes with an alert when it is close.

### S2. Inbound SMS has no duplicate check or reply throttle (Medium)

**Fixed in `de63535`.**

`Inbound` in `backend/internal/service/sms.go`. A provider retry is processed
twice, and a phone with an auto-responder can loop with the help reply.
**Fix:** deduplicate on the provider's message ID and limit replies per
number.

### S3. The dashboard's live feed survives sign-out (Medium)

**Fixed in `103a129`.** An open feed re-checks its session every 30 seconds.

`live` in `backend/internal/web/app_dashboard.go:175`. The session is checked
when the WebSocket opens and never again, so the feed keeps flowing after
logout, deactivation or company suspension. **Fix:** close a user's sockets
when their sessions are revoked, or re-check the session periodically.

### S4. Legacy SVG signatures are still accepted (Low)

**Fixed in `45c9578`.**

`uploadSVGSignature` in `backend/pkg/storage/pod.go:76`. The file is stored
unsanitised. The sandbox CSP that `serveMedia` sets on the response contains
it today, and the driver app only sends PNG. **Fix:** remove the path.

### S5. Account and shipment enumeration (Low)

**Fixed in `56a3133`.** Login, sign-up and track-by-phone answer alike; code checks are atomic.

- Driver login answers "belongs to an office account", and staff login
  answers "Drivers sign in through the driver app", before checking the
  credential.
- Sign-up says when an email or phone is already registered.
- Track-by-phone's "too many codes" reveals that a number has shipments.
- The verification code's attempt counter and its 3-an-hour limit are not
  updated atomically, so concurrent requests can slip past both.

### S6. Four write queries without a tenant filter (Low)

**Fixed in `36446b6`.**

`SetCustomerConfirmation`, `SetCustomerFeedback`, `SetShipmentConfirmToken`
and `AcknowledgeShipmentAlerts` update by ID alone. Their IDs come from
trusted lookups today, so this is not exploitable as written, but it breaks
the rule in `AGENTS.md` that every query is scoped by tenant, and the next
caller may not be as careful.

## Mobile: the driver app (read)

There is no native app. "Mobile" is the PWA in
`backend/internal/web/static/driver`.

| ID | Severity | Finding | Where | Fix | Status |
|---|---|---|---|---|---|
| M1 | High (product) | GPS is recorded only while the app is open and in the foreground. The live map, route-deviation alerts and "driver quiet" alerts degrade when the phone sleeps. The README acknowledges this. | Platform limit of a PWA | Wrap the app (for example Capacitor with background location), or describe tracking as "while the app is open". | Wording only, in `05611d9`: the product says "while the driver app is open". Background GPS still needs a native wrapper. |
| M2 | Medium | Start-up waits on the network. `boot()` awaits `/driver/config` with a 30-second timeout before drawing anything, and the service worker is network-first with no timeout. On weak signal the driver sees "Loading…" although all the data is already on the phone. | `static/driver/app.js:1451`, `static/driver/sw.js` | Draw from the cached config and refresh it in the background. Make the service worker cache-first or give it a short timeout. | Fixed in `c6b2fa8` |
| M3 | Medium | The delivery form is not saved as a draft. Low-memory Android phones often reload the page when the camera opens, which would lose the name, signature and photos. | `static/driver/app.js:1201` onwards | Save the form to IndexedDB as it is filled in. Test on a cheap phone first. | Fixed in `c6b2fa8` |
| M4 | Low | Refusal and "held for review" notices are kept in memory only (`S.notices`). A reload loses them, so the driver never learns a delivery was held. | `static/driver/app.js:525` | Store notices in IndexedDB until dismissed. | Fixed in `c6b2fa8` |
| M5 | Medium | When a company is suspended, sync returns 403. The app handles only 401, so it retries forever and shows nothing. | `static/driver/app.js:496-507` | Handle 403 with a message telling the driver to contact the office. | Fixed in `c6b2fa8` |
| M6 | Low | The home-screen icons are SVG only. iOS ignores SVG touch icons. Check Android's install prompt as well. | `static/driver/index.html`, `manifest.webmanifest` | Add PNG icons (180, 192, 512 px). | Fixed in `c6b2fa8` |
| M7 | Low | The signature canvas is not resized when the phone rotates. | `static/driver/app.js` | Resize on `orientationchange`, keeping the drawing. | Fixed in `c6b2fa8` |
| M8 | Low | The server cuts requests at 2 minutes while the app waits 3 minutes for a proof-of-delivery upload. | `backend/cmd/server/main.go:176`, `static/driver/app.js:480` | Make the two agree. | Fixed in `c6b2fa8` |

What the driver app already does well: the outbox survives a forced
sign-out, signing out warns about unsent items, Amharic and English, a
battery saver, a request for persistent storage, and proofs of delivery sent
one or two at a time.

## Web: dashboard and public pages (read)

| ID | Severity | Finding | Where | Status |
|---|---|---|---|---|
| W1 | High (product) | English only. The dashboard, customer SMS, the tracking page and the confirmation page have no Amharic. Only the driver app does. | Templates, `service/notify.go` | Open: needs a native Amharic reviewer. |
| W2 | Medium | The shipment page does not update live. It shows a toast with a "Reload" button, because `shipment.html` has no `data-live-region`. After the WebSocket reconnects, only the alert badge is refreshed, not the live regions. | `templates/app/shipment.html`, `static/app.js:360-364` | Fixed in `0a270db` |
| W3 | Medium | An alert's read state is shared by the whole company. One dispatcher opening the list clears the badge for everyone. | `sqlc/queries/alerts.sql` | Fixed in `0a270db` |
| W4 | Medium (product) | A shipment has one "customer". Shipper and consignee are not separate, so tracking texts, the confirmation link and the driver's Call button all go to one phone. | Schema, `service/shipment.go` | Open: a schema change. |
| W5 | Medium | A company cannot cancel its subscription. Renewal invoices issue automatically, an unpaid one makes the account read-only with a standing debt, and only the operator can void it. | `service/billing.go` | Fixed in `05611d9`: an admin turns off renewal. |
| W6 | Low | Staff who have a phone but no email cannot reset their own password. Reset is by email only. | `service/accounts.go` | Fixed in `56a3133` |
| W7 | Medium (product) | A phone number is unique across all companies (`idx_users_phone_unique`), so a driver cannot work for two companies. The error does not explain why. | Migrations, `GetUserByPhone` | Error explained in `56a3133`. A number still belongs to one account. |
| W8 | Medium | The quota counts messages, not SMS segments. Long or Amharic texts are several segments each, so they cost more than is billed. | `service/sms.go` | Fixed in `de63535` |
| W9 | Low | Name length limits count bytes. Amharic names get about a third of the intended room. | `service/shipment.go:154`, `service/users.go:42` | Fixed in `0a270db` |

What the web side already does well: light, fast pages; a skip link and ARIA
labels; dark mode; reduced-motion support; a guard against double submits;
debounced live refresh; responsive breakpoints at 1100, 900 and 600 px.

## Smaller logic gaps (read)

| ID | Finding | Where | Status |
|---|---|---|---|
| L1 | A proof of delivery is accepted straight from `assigned`, leaving `picked_up_at` empty. | `model/status.go`, `service/pod.go` | Fixed in `36446b6` |
| L2 | Reopening a shipment skips the read-only check and the per-driver load limit. | `service/shipment_status.go:357` onwards | Fixed in `36446b6` |
| L3 | A vehicle can be on two trips at once, and its capacity is not checked against cargo weight. | `service/vehicles.go`, `service/dispatch.go` | Fixed in `36446b6` |
| L4 | Quota and limit checks run outside the lock, so concurrent requests can exceed a limit by one. | `service/users.go`, `service/sms.go` | Fixed in `36446b6` (drivers); the text ceilings stay soft. |
| L5 | A verification SMS is retried for up to about 21 minutes, although the code expires in 10. | `service/sms.go`, `service/verification.go` | Fixed in `de63535` |
| L6 | The billing day drifts: 31 January, then 28 February, then 28 March. | `PeriodEnd` in `model/plan.go` | Fixed in `36446b6` |
| L7 | Job queries use `LIMIT 1000` without `ORDER BY`, so which rows are handled is arbitrary under load. | `sqlc/queries`, `service/jobs.go` | Fixed in `36446b6` |
| L8 | `ListDriverMessagesSince` stops at 200 rows and the cursor then jumps past the rest, so overflow is lost. | `sqlc/queries/shipment_events.sql` | Fixed in `2b3fc83` |
| L9 | GPS points for a shipment that was reassigned while the driver was offline are discarded. | `service/tracking.go:167` | Fixed in `2b3fc83` |
| L10 | `CompleteSignup` consumes the verification code outside the transaction, so a failure afterwards burns the code. | `service/tenants.go:173` | Fixed in `56a3133` |

## Operations and scale (read)

| ID | Finding | Recommendation | Status |
|---|---|---|---|
| O1 | One machine holds the app, the database and the uploaded photos. Backups land on the same machine unless someone copies them off, are unencrypted, and contain the app secret. There is no point-in-time recovery. | Ship backups off the machine, encrypted. Consider WAL archiving. | Open: infrastructure. |
| O2 | No alert rules are provisioned: `provisioning/alerting/` holds only `.gitkeep`. Nobody is told when jobs fail or the SMS queue stalls. | Add rules for job failures, outbox age, error rate and disk space. | Fixed in `3654d0e` and `b6cfd41` |
| O3 | The live-map query (`GetLatestDriverLocations`) scans seven days of GPS points per company on every load, and points are inserted one row at a time, up to 5000 per sync. | Fine at today's size. Add a last-position table and batch inserts before a fleet reaches hundreds of drivers. | Open: not needed at today's size. |
| O4 | Rate limits and the WebSocket hub are in memory, so the app must run as a single instance. This is documented. | No action until a second instance is needed. | Open: documented. |
| O5 | Leftovers: the unused `device_tokens` table from the retired native app, duplicate indexes, and login by email on `lower(email)` with no matching index. | Drop the table and duplicates; add an index on `lower(email)`. | Fixed in `3654d0e`. `device_tokens` was already dropped by the `product_core` migration. |
| O6 | The real telebirr, CBE Birr and Ethio telecom contracts are unconfirmed, so production can only take bank transfers. This is a documented go-live blocker. | Confirm each contract, then change the adapter and the sandbox together. | Open: needs the providers. |

## What is solid

- Transactions with row locks around every state change.
- SMS and email outboxes with leases and retries.
- Idempotent client IDs for everything the driver app sends.
- CSP (`script-src 'self'`), CSRF protection, hashed session tokens, rate
  limits per IP.
- Tenant filters on every read that was traced.
- The append-only audit trail and shipment history.
- The held proof-of-delivery review flow.
- A broad unit and integration test suite.

## Suggested order of work

1. **Before any paying customer:** R1, R5, R6 and S1.
2. **Driver trust in the app:** R2, R3, R4, M2 and M5.
3. **After a session on a low-end Android phone:** M3, M4, M6, M7, M8.
4. **Before go-live:** O1, O2, R7, S2, S3, and W1 for customer-facing text.
5. **Product decisions to make:** M1 (background GPS), W4 (shipper and
   consignee), W5 (cancelling a subscription), W7 (drivers at two companies).
6. **When convenient:** S4 to S6, the remaining W items, L1 to L10, O3, O5.

## Reproduction notes

The probe test was removed after the audit. To recreate it, add a file under
`backend/tests/integration/` using the helpers in `setup_test.go`, and run it
with `go test -tags integration ./tests/integration/ -run <name>`.

| Finding | Steps |
|---|---|
| R1 | Send five wrong PINs for a driver and expect 403. Expire the lock (set `locked_until` in the past). Send one wrong PIN, then the right one. Both are refused. |
| R2 | Assign a shipment to a driver. Sync a "start trip" status whose device time and recorded time are 90 seconds behind the server. The result is `STALE`. |
| R3 | Sync two statuses (in transit, then arrived) with client IDs, using recorded times after the booking. Send the identical request again. The first comes back `STALE`. |
| R4 | Have the driver send a note, then sync and read `messages`: the note is there under the driver's name. Cancel a shipment from the dashboard with no note and sync: a message with empty text arrives. |
| R5 | As an admin, create a staff user with an email and do not confirm it. Post that email to `/forgot`. The mail outbox holds a reset message for it. |
| R6 | Encode a blank 8000×8000 PNG and send it as a delivery photo. It is accepted; compare the process's memory statistics before and after. |
| R7 | `go test -tags integration ./tests/integration/ -run TestGatewayFaults`. |

One trap when writing these: status times earlier than the shipment's booking
are stale by design, so record times after booking or the test reports a
conflict that is not a bug.
