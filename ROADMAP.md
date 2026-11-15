# Roadmap

What to do next, in order. Written on 2026-11-14 from a survey of the
repository at `7320639`. Tick the boxes as work lands, and delete a section
once it is done: this file should only ever hold what is still ahead.

1. [Decisions](#decisions-to-make-first) that shape the release.
2. [The next release](#the-next-release-a-clean-baseline): a settled name,
   a codebase anyone can open and follow, and commit attribution corrected.
3. [After the release](#after-the-release-toward-the-first-paying-customer):
   toward the first paying customer.

## Where things stand

- One Go application in `backend/`: about 25,400 lines of Go and 12,900
  lines of tests (210 unit tests, 95 integration tests, 8 end-to-end tests),
  plus 53 templates and a 1,663-line driver app in plain JavaScript.
- CI runs vet, unit tests, gofmt, `sqlc diff`, govulncheck, shellcheck, the
  integration tests and the end-to-end tests against staging.
- The audit in `docs/audit-2026-11.md` is fixed except for W1, W4, O1, O3,
  O4 and O6.
- The repository is public and all rights are reserved (`LICENSE`). It has
  no releases or tags, and ten old branches on GitHub that are all merged
  into `main`.
- The product and the repository were renamed Kiflet on 2026-11-15. The
  checks and registrations that make the name final are still open: see
  [Finish the rename](#1-finish-the-rename).

## Decisions to make first

Each of these changes what the release does. The recommendation is what the
rest of this file assumes. The ones marked were settled on 2026-11-15.

| Decision | Recommendation | Why |
|---|---|---|
| Move everything out of `backend/` to the repository root | Yes | There is no frontend or mobile directory beside it any more, and the module path in `go.mod` has never included `backend`. |
| Replace the 21 migrations with one baseline schema | **Decided:** yes. No database holds data that must be kept | See [Migrations](#7-migrations). |
| Twilio SMS adapter | Remove | Nothing uses it and the provider sandbox doesn't cover it. It comes back from git history if needed. |
| Correct the author name and email while rewriting history | Yes | 57 commits carry a misspelled name, and all carry a personal email address in a public repository. History is being rewritten anyway. The cost is under [Commit attribution](#9-commit-attribution). |
| Release number | `v0.9.0` now, `v1.0.0` with the first paying customer | The product isn't live yet. |

## The next release: a clean baseline

No new features. Each numbered part is its own pull request, changes no
behaviour, and must pass CI. Do them in this order. The rename is in, and
part 1 holds what is left of it. The later parts are easier once the
earlier ones are in, and the history rewrite has to be last.

### 1. Finish the rename

The product was renamed Kiflet on 2026-11-15: the code, the documents and
the repository. `CHANGELOG.md` lists what moved. The name was taken "for
now", so this part is what makes it final, or replaces it.

**Kiflet (ቅፍለት)** is a caravan: loaded animals and their drivers
travelling together, the way goods crossed the country before trucks did.
It is what a trucking company runs today. The Latin spelling has "fleet"
nearly hidden in it, and at six letters it fits an SMS sender ID, which
allows eleven.

Checked on 2026-11-14:

- A web search found no product, company or app named Kiflet, in Ethiopia
  or elsewhere.
- whois has no record of `kiflet.com`, `.io` or `.co`, and `kiflet.et`,
  `.com.et`, `.app` and `.africa` have no DNS entry.
- No GitHub account or organization is named `kiflet`.

Still to check, before anything is registered under the name:

- [ ] Say it to a few people in the trade. Does ቅፍለት mean to them what it
      should, and is it easy to say and to type? "Kiflet" writes ቅ with a
      K, as most people would; "Qiflet" is the stricter spelling.
- [ ] Search the trademark register (EIPA) and the trade name register.
- [ ] Search the Play Store and the App Store.

Once those pass:

- [ ] Take the names: the domains (`kiflet.et` and `kiflet.com` at least)
      and the GitHub organization `kiflet`. Creating the organization holds
      the name even while the repository stays where it is.
- [ ] Ask Ethio telecom for the sender ID `Kiflet`, give telebirr and CBE
      Birr the merchant name, and set up `support@` and `no-reply@` on the
      domain.

Left over from the old name:

- [ ] In any other clone:
      `git remote set-url origin https://github.com/Natnael-Alemayehu/kiflet.git`.
      Its Docker volumes and images still carry the old project's name;
      remove them.

If Kiflet fails a check, the next choices:

| Name | Meaning | What the checks found |
|---|---|---|
| Sirara (ሲራራ) | The long-distance caravan trade, as in ሲራራ ነጋዴ | No product by the name. `sirara.et` has no DNS entry; `sirara.com` and the GitHub name are taken. |
| Dersual (ደርሷል) | "It has arrived": what a proof of delivery says | No product by the name, and none of six domains tried has a DNS entry. The GitHub name is taken. Harder to spell in Latin letters. |
| Rikikib (ርክክብ) | Handover: the moment the product records | The same as Dersual, and more awkward in Latin letters. |

Ruled out, so nobody tries them again: Kella (the Ministry of Transport and
Logistics has a Kella app for truck terminals), Finot (FINOT Technologies is
a software company in Addis Ababa), Sinq (Sinq Technologies sells
supply-chain software), Adera (every domain is taken, `adera.et` included),
Mahtem (`mahtem.et` is in use), Feleg (a given name, and its `.com` and
`.app` are taken), and Chinet, Tolo, Garri and Woye (freight apps).

Renaming again costs little while nobody is live. The first rename was
these commands and no edits by hand, 604 lines in 126 files:

```bash
git grep -lzi kiflet -- . ':!backend/internal/db' ':!CHANGELOG.md' ':!ROADMAP.md' |
  xargs -0 sed -i 's/Kiflet/Newname/g; s/kiflet/newname/g; s/KIFLET/NEWNAME/g'
cd backend
git mv deployments/grafana/dashboards/kiflet.json \
       deployments/grafana/dashboards/newname.json
git mv deployments/grafana/provisioning/dashboards/kiflet.yml \
       deployments/grafana/provisioning/dashboards/newname.yml
make sqlc
```

Then: a migration like `20261115000000_purge_setting`, so that a database
made under the old name can still purge; `gofmt -w .` if the new name isn't
six letters long; a `CHANGELOG.md` entry; and every test, staging included.
The three things that hold state are the reason to settle this before the
first customer: the purge setting in the database, the volumes Compose
names after the project, and the database on each driver's phone.

### 2. The front door

What someone sees before reading any code. The licence, `SECURITY.md`,
`CONTRIBUTING.md`, the `docs/` folder and the README's screenshots are in.
What is left:

- [ ] On GitHub: turn on private vulnerability reporting (Settings, Code
      security), which `SECURITY.md` tells reporters to use; add topics;
      delete the merged branches; and protect `main` so CI must pass.
- [ ] `CHANGELOG.md`: add each remaining part of this release as it lands.
- [ ] Retake the two screenshots in `docs/images/` when the overview or the
      driver app's first screen changes.

### 3. Layout

- [ ] Move the contents of `backend/` to the root with `git mv`, in a pull
      request that does nothing else. Then fix every path that still says
      `backend/`: `git grep -n 'backend/'` lists them, in `.gitignore`,
      `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `LICENSE`, the documents
      in `docs/`, `.github/workflows/ci.yml` (the working directory and the
      cache paths) and `scripts/backup.sh`. Delete the root `Makefile`,
      which only forwards to `backend/`.
- [ ] Put the development files together: `docker-compose.yml` and
      `sandbox.env` sit at the top of `backend/` while production's compose
      file is in `deployments/`. Move them to `deployments/dev/` and point
      the Makefile at them.

### 4. Dead code and leftovers

Found by searching for exported names nothing refers to. No linter was
available for the survey, so the first run of the linter in part 8 will add
to this list.

- [ ] `pkg/websocket`: the hub was built for per-shipment and per-driver
      rooms that the dashboard never used. Unused outside tests:
      `BroadcastToDriver`, `BroadcastToShipment`, `BroadcastAll`,
      `SubscribeToRoom`, `UnsubscribeFromRoom`, `RoomCount`,
      `TenantClientCount`, `IsConnected`, and all of `room.go` and
      `message.go` (the app sends its own messages). Remove them with
      `room_test.go`, `message_test.go` and the matching parts of
      `hub_test.go`.
- [ ] `pkg/storage/s3.go`: `GetPublicURL` is never called.
- [ ] Twilio, as decided: `pkg/sms/twilio.go` and its test, the provider
      constant, three settings in `internal/config`, their wiring in
      `internal/app`, and the lines in `.env.example`.
- [ ] `.gitignore`: drop rules for things that no longer exist, and keep one
      rule per line of reasoning.

### 5. Files that are too large to read

Split by the sections the files already mark. Moves only, no edits.

- [ ] `internal/service/billing.go` (2,124 lines) has eight banner comments.
      Make each a file: `billing_standing.go`, `billing_plans.go`,
      `billing_choose.go`, `billing_invoices.go`, `billing_payments.go`,
      `billing_operator.go`, `billing_job.go`, with the helpers staying in
      `billing.go`.
- [ ] `internal/web/app_admin.go` (869 lines) and `app_shipments.go` (824):
      split along their handler groups the same way.
- [ ] `internal/web/static/driver/app.js` (1,663 lines, one file, no tests):
      split into ES modules, which need no build step: `store.js`
      (IndexedDB), `sync.js` (outbox and sync), `gps.js`, `text.js` (English
      and Amharic), `signature.js`, and one module per screen. Add the new
      files to the service worker's `SHELL` list;
      `TestDriverShellFilesExist` then checks them.

### 6. Tests named for what they test

- [ ] `tests/integration/audit_test.go` holds 27 tests grouped by where the
      findings came from, not by subject. Move each to the file for its
      subject (`accounts_test.go`, `billing_test.go`, `dispatch_test.go`,
      `driver_app_test.go`, and a new `sms_test.go` and `sync_test.go`),
      keeping the finding's ID in the test's comment.
- [ ] `tests/integration/logic_test.go` and `internal/service/logic_test.go`
      are named after a round of fixes. Rename them by subject too.
- [ ] `TestRateLimits` still fails on a busy machine. Its 31 sign-ins must
      land inside one clock minute, and it only waits for a fresh minute
      when under 20 seconds are left; on a loaded machine they took 34.
      Give the limiter a clock the test can set, or have the test retry
      once when it crossed a minute.
- [ ] Driver app: a small set of headless-browser tests run with the
      end-to-end suite: sign in, receive a load, deliver offline, sync, and
      restore a delivery draft after a reload. This is the only part of the
      product with no automated test of its own.

### 7. Migrations

There are 21 migration files. They tell the story of how the schema got
here, including a table that was created and later dropped
(`device_tokens`), and names like `logic_fixes` and `product_core` that
mean nothing to a new reader.

- [ ] If no database holds data that must be kept: replace them with one
      `…_baseline.up.sql` holding the schema as it is now, generated with
      `pg_dump --schema-only` from a freshly migrated database and tidied.
      Prove it with a test that migrates one database the old way and one
      the new way and compares the schemas.
- [ ] If one does: leave the files alone and add `docs/schema.md`, a short
      guide to the tables, instead.

### 8. Checks that keep it clean

- [ ] golangci-lint in CI with `staticcheck`, `unused`, `errcheck` and
      `ineffassign`. Fix or explicitly excuse what the first run reports.
- [ ] `node --check` on every JavaScript file in CI. It was run by hand for
      each fix so far.
- [ ] A secret scan (gitleaks) over the whole history, once now and then in
      CI. A filename search of the history found only example files, but
      contents weren't scanned.
- [ ] Dependabot for Go modules, GitHub Actions and the Docker base images.
- [ ] Report integration-test coverage of `internal/service` and
      `internal/web`. Unit coverage shows 5.8% and 4.2% for them only
      because their tests are the integration tests.

### 9. Commit attribution

GitHub lists Claude as a contributor because 28 commits on `main` end
with a `Co-Authored-By: Claude … <noreply@anthropic.com>` line. Removing
it from the list means removing those lines, which means rewriting history
and replacing `main` on GitHub. Do it once, as the last step before the tag,
when no pull request is open.

Where Claude appears today:

| Where | What |
|---|---|
| 28 commit messages | The co-author line: 14 on the fixes of 23 August, 14 on the audit fixes |
| 5 commit messages | "the Fable audit" and `fable-audit.md` |
| Pull request #2 | "Generated with Claude Code" at the end of its description |

No file names Claude any more: the audit is `docs/audit-2026-11.md`.

Steps:

- [ ] **Stop new ones.** In Claude Code's user settings, not in a file in
      this repository, set `"attribution": {"commit": "", "pr": ""}`
      (older versions: `"includeCoAuthoredBy": false`).
- [ ] **Back up.** `git clone --mirror` the repository somewhere safe, and
      keep it until everything below is verified.
- [ ] **Rewrite**, in a fresh clone, with `git filter-repo`
      (`sudo dnf install git-filter-repo`):

  ```bash
  git filter-repo --message-callback '
  import re
  message = re.sub(rb"(?im)^co-authored-by: claude[^\n]*\n?", b"", message)
  message = message.replace(b"the Fable audit", b"the audit")
  message = message.replace(b"fable-audit.md", b"docs/audit-2026-11.md")
  return message.rstrip() + b"\n"
  '
  ```

  To correct the author name and email in the same pass, add
  `--mailmap <file>` with one line per old identity, mapping it to the
  right name and GitHub's `…@users.noreply.github.com` address.
- [ ] **Verify.** `git log --format=%B | grep -ciwE "claude|fable"` prints
      0, and `git rev-parse HEAD^{tree}` equals the old `main`'s tree: the
      files are identical, only messages changed.
- [ ] **Fix the audit's hashes.** It quotes 12 commit hashes. Map them with
      `.git/filter-repo/commit-map`, and check each new one is an ancestor
      of `main`.
- [ ] **Publish.** `git push --force origin main`, then delete the merged
      branches on GitHub; `product-consolidation` still holds the old
      commits.
- [ ] **Pull request #2.** Edit its description to drop the last line.
- [ ] **Check GitHub a day later.** The contributor list is cached.

What it costs:

- Without the mailmap, the 44 commits before 23 August keep their hashes
  and every later one changes. With it, they all change.
- Three commits made on GitHub (two README edits and the merge of pull
  request #2) lose their "Verified" badge; with the mailmap, the merge of
  pull request #1 does too.
- Every other clone has to be cloned again. Today there are no forks and no
  other collaborators, which is why this belongs in this release and not a
  later one.
- The old commits stay reachable through pull request #2's page. Only
  GitHub Support can remove them, and the contributor list doesn't need it.

### 10. Tag it

- [ ] `CHANGELOG.md` entry, tag `v0.9.0` on the rewritten `main`, and a
      GitHub release. Tag after the rewrite, never before.

## After the release: toward the first paying customer

In order. The first three can run side by side.

1. **Start provider onboarding as soon as the name is settled** (audit O6).
   telebirr, CBE Birr and Ethio telecom each need a contract, and the wait
   is theirs, not ours. Apply under the new name: the sender ID and the
   merchant names are slow to change afterwards. Until the providers
   answer, production takes bank transfers only. `README.md` lists what to
   settle with each.
2. **A session on a cheap Android phone.** The audit never saw the driver
   app on a real device. Deliver a load with the camera, on weak signal,
   with the phone locked in between. Fix what it shows before anything
   else in the driver app.
3. **Backups that survive the machine** (O1). Encrypt them, copy them off
   the server, and restore one as a drill. Small, and the one way a pilot
   could lose a customer's data.
4. **Amharic for customers, then for the office** (W1). First the tracking
   page, the confirmation page and the texts customers get; then the
   dashboard. It needs a native speaker, who should also review the
   driver app's newest strings.
5. **A pilot with one trucking company**, paying by bank transfer. Everything
   after this point should be reordered by what the pilot shows.
6. **Shipper and consignee as separate people** (W4). A schema change, and
   the most likely first request from a real customer: today the tracking
   texts, the confirmation link and the driver's Call button all go to one
   phone.
7. **Tracking with the screen off** (M1), if the pilot asks for it: wrap the
   driver app in Capacitor with a background-location plugin. The web code
   stays the app.
8. **Later:** CSV import of shipments (FR-013), a last-position table for
   the live map (O3), a second app instance (O4), a public API and webhooks
   (FR-046, FR-047).

## Decided, and when to revisit

- **The name Kiflet, for now** (2026-11-15). Revisit if one of the checks
  in part 1 fails. Part 1 has the fallbacks and what a second rename takes.
- **No GitHub organization yet** (2026-11-15). The repository is `kiflet`
  under the personal account. Revisit when someone joins, or before the
  name is used in public: the organization name `kiflet` was free on
  2026-11-14 and nothing holds it. Moving there changes the module path
  once more, which is one `go mod edit` and one `sed` over the imports.
- **One codebase** (2026-11-14). Splitting into separate backend, web and
  mobile repositories was considered and set aside: about 96 routes render
  pages and 8 return JSON, so a separate web app means rebuilding the
  dashboard and an API for all of it. Revisit when dedicated web or mobile
  developers join, when the product needs screens that server-rendered
  pages can't deliver, or when a public API is being built anyway.

## Considered and not planned

- **Moving `pkg/` under `internal/`.** Nothing outside this module imports
  it, but the move touches every file for no change a reader would feel.
- **Rewriting old commit messages** beyond the lines in part 9. The early
  history is rough, and it is still the history. That includes the
  product's first name: old commits keep it.
