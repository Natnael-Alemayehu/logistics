# Roadmap

What to do next, in order. Written on 2026-11-14 from a survey of the
repository at `f540720`. Tick the boxes as work lands, and delete a section
once it is done: this file should only ever hold what is still ahead.

1. [Finish the redesign](#finish-the-redesign): the rest of the pages, and
   the features the design added.
2. [Finish the rename](#finish-the-rename): the checks and registrations
   that make the name final.
3. [Toward the first paying customer](#toward-the-first-paying-customer).

## Where things stand

- One Go application, at the repository root: about 25,000 lines of Go and
  12,800 lines of tests (175 unit tests, 96 integration tests, 9 end-to-end
  tests, one of them the driver app in headless Chrome), 53 templates, a
  driver app in plain JavaScript (19 ES modules, no build step), and the
  schema in one migration.
- CI runs vet, golangci-lint, unit tests, gofmt, `sqlc diff`, govulncheck,
  shellcheck, `node --check`, a secret scan of the whole history, the
  integration tests and the end-to-end tests against staging. Dependabot
  proposes updates weekly.
- The audit in `docs/audit-2026-11.md` is fixed except for W1 on the
  dashboard and O6.
- The repository is public and all rights are reserved (`LICENSE`), with
  private vulnerability reporting on and `main` protected so CI must pass.
  The first release is `v0.9.0`, of 2026-11-15. History was rewritten that
  day: every commit has the right author name and GitHub's no-reply
  address, and no co-author lines.
- The product and the repository were renamed Kiflet on 2026-11-15. The
  checks and registrations that make the name final are still open: see
  [Finish the rename](#finish-the-rename).

## Finish the redesign

The dashboard is being rebuilt to the design called Direction D, made on
the Claude Design canvas: a light theme ("Desk"), a dark one ("Night"),
Geist type and lucide icons. The shell, the overview, the shipment list, a
shipment's page and the booking form are done. Each part below is a pull
request; the design's pages are named after it.

- [ ] **A shipment's documents and charges.** Two tabs on a shipment's
      page: documents the office uploads (waybill copies, customs papers,
      invoices) to list and download, and charges as line items with a
      total (D-Shipment).
- [ ] **Drivers and vehicles.** A driver's licence expiry, and a vehicle's
      insurance and annual inspection (ቦሎ) dates, each with a reminder
      before it runs out; a "papers due in 30 days" figure and a banner
      for what has lapsed (D-Drivers, D-Vehicles).
- [ ] **Alerts and their rules.** The alerts page with rules a company
      sets: running late by more than a set time, a phone not heard from,
      a long stop, leaving the route; and who gets each alert (D-Alerts).
- [ ] **The company's pages.** Reports, team (with what each role can do),
      billing, settings (in sections) and help, with Telegram as a support
      channel (D-Reports, D-Team, D-Billing, D-Settings, D-Help).
- [ ] **The public pages.** Signing in by phone or email, the tracking page
      with the carrier's details, and finding a shipment by phone with a
      code, in English and Amharic (D-SignIn, D-Tracking, D-FindByPhone).
- [ ] **Amharic on the dashboard** (audit W1), after a native speaker has
      reviewed the words. The design's language switch waits for it.

## Finish the rename

The product was renamed Kiflet on 2026-11-15: the code, the documents and
the repository. `CHANGELOG.md` lists what moved. The name was taken "for
now", so this section is what makes it final, or replaces it. It is work
outside the repository.

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
- [x] Search the Play Store and the App Store. Searched on 2026-11-16:
      the App Store has no app of the name in its Ethiopian, British or
      American catalogue, and a Play Store search returns only kitchen
      games, which it offers for a word it doesn't know.

Once those pass:

- [ ] Take the names: the domains (`kiflet.et` and `kiflet.com` at least)
      and the GitHub organization `kiflet`. Creating the organization holds
      the name even while the repository stays where it is.
- [ ] Ask Ethio telecom for the sender ID `Kiflet`, give telebirr and CBE
      Birr the merchant name, and set up `support@` and `no-reply@` on the
      domain.

Left over from the old name:

- [ ] Any other clone: clone again, since history was rewritten on
      2026-11-15. Its Docker volumes and images still carry the old
      project's name; remove them.

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
git grep -lzi kiflet -- . ':!internal/db' ':!CHANGELOG.md' ':!ROADMAP.md' |
  xargs -0 sed -i 's/Kiflet/Newname/g; s/kiflet/newname/g; s/KIFLET/NEWNAME/g'
git mv deployments/grafana/dashboards/kiflet.json \
       deployments/grafana/dashboards/newname.json
git mv deployments/grafana/provisioning/dashboards/kiflet.yml \
       deployments/grafana/provisioning/dashboards/newname.yml
make sqlc
```

Then: a migration that defines `append_only()` again with the new
setting's name, so that a database made under the old name can still purge
(the first rename's was `20261115000000_purge_setting`, now in git history); `gofmt -w .` if the new name isn't
six letters long; a `CHANGELOG.md` entry; and every test, staging included.
The three things that hold state are the reason to settle this before the
first customer: the purge setting in the database, the volumes Compose
names after the project, and the database on each driver's phone.

## Toward the first paying customer

In order. The first two can run side by side.

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
3. **A native speaker reads the Amharic, then Amharic for the office**
   (W1). The texts customers get and their tracking and confirmation
   pages are in Amharic, drafted without one: `internal/text/am.go` is
   the whole of it, beside `en.go`. The same reader should go through the
   driver app's `static/driver/text.js`, and say whether customers expect
   dates in the Ethiopian calendar, which they are not in. Until then no
   company should choose Amharic in Settings. The dashboard comes after,
   through the same catalogue, with each user's own language
   (`users.language`).
4. **A pilot with one trucking company**, paying by bank transfer. Before
   it starts, production's backups go off the machine: choose the bucket's
   provider, make two age keys held by two people, fill in `backup.env`,
   and run the drill in `docs/deploy.md` once. Decide whether to turn on
   `WAL_ARCHIVE` and add `ship-wal.sh` to cron. Everything after this point
   should be reordered by what the pilot shows.
5. **Tracking with the screen off** (M1), if the pilot asks for it: wrap the
   driver app in Capacitor with a background-location plugin. The web code
   stays the app.
6. **Built ahead of need:** the public API and webhooks (FR-046, FR-047),
   two app instances (O4, `APP_REPLICAS`) and restoring to the minute
   (`WAL_ARCHIVE`, the rest of O1). Turn the last two on for the pilot if
   it can't afford a restart's gap or a lost day.

## Decided, and when to revisit

- **`v1.0.0` with the first paying customer** (2026-11-15). Until then
  releases are `0.x`; `v0.9.0` is the first.
- **The driver app is ES modules with no build step** (2026-11-16): the
  files in `static/driver` are what the phone runs, and `index.html`
  preloads every module. Revisit if it needs packages from npm, or types
  checked before it ships.
- **The name Kiflet, for now** (2026-11-15). Revisit if one of the checks
  in [Finish the rename](#finish-the-rename) fails. That section has the
  fallbacks and what a second rename takes.
- **No GitHub organization yet** (2026-11-15). The repository is `kiflet`
  under the personal account. Revisit when someone joins, or before the
  name is used in public: the organization name `kiflet` was free on
  2026-11-14 and nothing holds it. Moving there changes the module path
  once more, which is one `go mod edit` and one `sed` over the imports.
- **One codebase** (2026-11-14). Splitting into separate backend, web and
  mobile repositories was considered and set aside: about 96 routes render
  pages and 8 return JSON, so a separate web app means rebuilding the
  dashboard and an API for all of it. Revisit when dedicated web or mobile
  developers join, or when the product needs screens that server-rendered
  pages can't deliver. The public API (2026-11-17) didn't change this: it
  calls the same services as the dashboard, from the same binary.

## Considered and not planned

- **Moving `pkg/` under `internal/`.** Nothing outside this module imports
  it, but the move touches every file for no change a reader would feel.
- **Rewriting old commit messages** beyond the co-author lines and audit
  references the rewrite of 2026-11-15 removed. The early
  history is rough, and it is still the history. That includes the
  product's first name: old commits keep it.
