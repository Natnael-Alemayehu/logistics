# End-to-end walkthrough

A manual pass over every flow, in the order a new customer meets them. The
automated version of the core flows is `make test-integration`.

## Setup

```bash
make db && make seed && make run
```

Open http://localhost:8080. SMS is simulated: every message appears under
Settings → Text messages, and the server log prints `[MOCK SMS]` lines. Links
in seeded messages use the URL from when `make seed` ran.

To use the driver app on a real phone, the phone must reach your machine, and
service workers need HTTPS except on `localhost`. A tunnel (for example
`cloudflared tunnel --url http://localhost:8080`) is the easiest way. Start the
app with `PUBLIC_URL` set to the tunnel address so that SMS links point there.

## 1. A company signs up

1. On the landing page, choose **Start a free trial**. Enter a company, city,
   your name, email, an Ethiopian mobile number (`09…` or `07…`) and an
   8-character password.
2. You land on an empty dashboard with a trial banner. The setup steps point
   to adding a vehicle, a driver and a first shipment.

Check: a second signup with the same email or phone is refused with a
field-level message.

## 2. Vehicles, drivers and the team

1. **Vehicles → Add vehicle**: plate `3-A12345 AA`, type, capacity.
2. **Drivers → Add driver**: name, phone, default vehicle. A one-time PIN is
   shown once (and texted to the driver).
3. **Team → Add person**: a dispatcher with an email. A temporary password is
   shown once.
4. Sign in as the dispatcher in a private window. You must choose a new
   password first. A dispatcher can book and manage shipments but gets 403 on
   Team, Settings and Audit log.

Check: the trial allows 5 drivers, and adding a sixth explains the limit.

## 3. Book a shipment

1. **New shipment**: pick origin and destination towns from the list. **Pin the exact
   drop-off on a map** narrows the delivery location check from the town to 500 m. Customer name and phone are required.
2. Assign the driver now or later. The driver's default vehicle fills in.
3. The shipment page shows the tracking number, the route on a map, the due
   time (estimated from distance) and the history.

Check:
- The customer got a "booked" SMS with a tracking link, and the driver got a
  "new shipment" SMS.
- **Book again** on any shipment pre-fills a copy.
- **Waybill** prints a one-page document.

## 4. The driver app

1. Open `/driver/` on a phone, or in a narrow browser window. Sign in with the
   driver's phone and PIN, then choose a new PIN.
2. The load appears under **Active**. Open it and tap **Start trip**.
3. Turn on airplane mode (or DevTools → Network → Offline). The status pill
   shows **No signal**. Tap **I've arrived**, then **Record delivery**: enter the
   recipient's name, have them sign, add a photo and confirm. The pill shows
   **1 waiting to send**, and the shipment shows as delivered on the phone.
4. Go back online. Within seconds the pill shows **All sent**.

Check, on the dashboard:
- The shipment is **Delivered**, with the signature, photo, recipient and a
  **Location verified** badge when the phone was within range of the
  destination.
- The history shows each step with its source (Driver app).
- The customer got "on its way", "arrived" and "delivered" SMS. The last one
  carries a confirmation link.

Also try:
- **Report a delay** or **Report a problem** with a reason. An alert appears
  on the dashboard at once, as a toast and on the bell.
- Send a message from the shipment page with **To the driver**. It appears in
  the app after the next sync.
- **Forgot your PIN?**: request a code, read it under Settings → Text
  messages, and set a new PIN.

## 5. Conflicts

1. Assign a shipment, let the driver sync, then take the phone offline.
2. On the phone, tap **Start trip**. It waits in the outbox.
3. On the dashboard, **Cancel shipment**.
4. Bring the phone back online.

Check:
- The shipment stays cancelled.
- The phone shows a **Not applied** notice.
- The shipment history records the update that was not applied, and when it
  was made.

A delay or problem reported this way also raises an alert, so the office sees
what the driver ran into. A proof of delivery is never discarded: it is kept
on a cancelled shipment, and the office is alerted.

## 6. Drivers without a smartphone

Settings → Text messages → **Simulate an incoming text**, from a driver's
number:

| Text | Effect |
|---|---|
| `ET 7K3M 1` | departed (in transit) |
| `ET 7K3M 2 traffic at Awash` | delayed, with the note |
| `ET 7K3M 3` | arrived |
| `ET 7K3M 5 flat tyre` | problem reported, with an alert |
| `help` or anything unparseable | a reply explaining the format |

`7K3M` stands for the last four characters of the tracking number. A text
from an unknown number changes nothing and gets no reply.

## 7. The customer

1. Open the tracking link from an SMS, or enter the number at `/track`. The
   page shows the status steps, due time, a coarse last-known position ("45 km
   from Adama") and public history. It never shows phone numbers or the
   driver's name.
2. `/track/phone`: enter the customer's phone, read the 6-digit code from the
   SMS log, and see all of their shipments.
3. Open the confirmation link from the delivery SMS. Confirm, or report a
   problem with a note. A reported problem raises an alert for the office.

## 8. Running the business

- **Overview**: KPIs, live map (green = online, amber = quiet, red = offline),
  what needs attention, recent activity. Updates arrive live.
- **Shipments**: tabs (unassigned, active, needs attention, delivered,
  cancelled), search and filters, CSV export.
- **Alerts**: late shipments, problems, drivers quiet for four hours while
  carrying cargo. Dismiss each one, or mark all read.
- **Reports**: deliveries per day with on-time share, on-time rate and transit
  time by driver, route and customer, and CSV for each section.
- **Settings**: company details, which SMS go out, the active-load limit per
  driver.
- **Audit log**: sign-ins, failed sign-ins and lockouts, credential resets,
  and every change to people, vehicles and settings, with before and after
  values.
- **Account**: profile, password, and sessions on other devices, which you
  can sign out.

## 9. The operator

Sign in as `admin@menged.et`. **Platform** lists every company with plan,
drivers and activity. Change a plan or driver limit, or suspend a company:
its users are signed out at once and can't sign back in until it is restored.

## 10. Security checks

- Five wrong passwords lock the account for 15 minutes. Further attempts from
  the same address are rate-limited.
- Deactivating a driver or staff member ends their sessions at once. The
  driver app returns to the sign-in screen on its next sync.
- One company's IDs in another company's URLs return 404.
- Forms posted from another site are refused (cross-origin protection).
- `/metrics` returns 404 unless `METRICS_TOKEN` is set and sent as a bearer
  token.
