# End-to-end walkthrough

A manual pass over every flow, in the order a new customer meets them. The
automated version of the core flows is `make test-integration`.

## Setup

```bash
make db && make seed && make run
```

Open http://localhost:8080. SMS, email and payments are stand-ins: what they
would send is printed in the terminal running `make run`, in a framed block
(`┌─ SMS to 0912345601`). Codes, passwords and links are only readable there;
Settings → Text messages lists every SMS but hides codes. Links in seeded
messages use the URL from when `make seed` ran.

To use the driver app on a real phone, the phone must reach your machine, and
service workers need HTTPS except on `localhost`. A tunnel (for example
`cloudflared tunnel --url http://localhost:8080`) is the easiest way. Start the
app with `PUBLIC_URL` set to the tunnel address so that SMS links point there.

## 1. A company signs up

1. On the landing page, choose **Start a free trial**. Enter a company, city,
   your name, email, an Ethiopian mobile number (`09…` or `07…`) and an
   8-character password, then **Continue**.
2. A code is texted to that number: read it in the server terminal
   (`your sign-up code is …`) and enter it. Nothing is created until the code
   comes back. **Send a new code** sends another (at most 3 an hour), and
   **Change your details** returns to the form with what you typed.
3. You land on an empty dashboard. A banner asks you to confirm your email
   address: open the `/verify-email/…` link printed in the terminal and the
   banner goes. The setup steps point to adding a vehicle, a driver and a
   first shipment.

Check: a wrong code is refused next to the field, and after five wrong tries
the code stops working. A second signup with the same email or phone is
refused with a field-level message.

Sign up again with another email and phone, choosing **Starter** under
**Plan** (or **Start with Starter** on the landing page). The trial still
starts, and you land on the Starter invoice instead of the dashboard. It asks
you to confirm your email before paying; section 9 continues from there.

## 2. Vehicles, drivers and the team

1. **Vehicles → Add vehicle**: plate `3-A12345 AA`, type, capacity.
2. **Drivers → Add driver**: name, phone, default vehicle. A one-time PIN is
   shown once (and texted to the driver).
3. **Team → Add person**: a dispatcher with an email. A temporary password is
   shown once, and texted if you gave a phone. The terminal also shows an
   email to the dispatcher saying who added them, with a link to confirm the
   address.
4. Sign in as the dispatcher in a private window. You must choose a new
   password first. A dispatcher can book and manage shipments but gets 403 on
   Team, Settings and Audit log.
5. Sign out and choose **Forgot your password?**. Enter the dispatcher's
   email: the page reads the same whether or not an account has that address.
   Open the `/reset/…` link from the terminal and choose a new password. You
   are signed in, and the dispatcher's other sessions end. The link works
   once, within the hour.

Check: the trial allows 10 drivers, and adding an eleventh explains the
limit.

## 3. Book a shipment

1. **New shipment**: pick origin and destination towns from the list. **Pin the exact
   drop-off on a map** narrows the delivery location check from the town to 500 m. Customer name and phone are required.
2. Assign the driver now or later. The driver's default vehicle fills in.
3. Optionally set a **Pickup** time under Timing. The due time then counts
   from the pickup rather than from now.
4. The shipment page shows the tracking number, the route on a map, the
   pickup and due times and the history.

Check:
- The customer got a "booked" SMS with a tracking link. The driver got a "new
  shipment" SMS with the pickup time and the exact text to send when they
  leave ("text ET 7K3M 1 to 6060").
- A load whose pickup is still ahead is never shown as late. Two hours after
  a missed pickup, a **Not collected** alert appears.
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
  on the dashboard at once, as a toast and on the bell. Report it again with
  another reason (**Update the delay**): the history shows **Delay updated**
  and the alert describes the new cause. The customer is not texted twice.
- Send a message from the shipment page with **To the driver**. It appears in
  the app after the next sync. If the app hasn't synced for 15 minutes, the
  message goes by SMS as well, and the history says so.
- Dismiss a **Past due time** alert. It stays dismissed. A new due time that
  passes raises a new alert.
- **Forgot your PIN?**: request a code, read it in the server terminal, and
  set a new PIN.

### On the road

Chrome's DevTools can stand in for the road: **Sensors** sets the phone's
position, and **Network** throttles or cuts the connection.

- **Distance to the drop-off.** With a position set (e.g. Addis Ababa,
  9.03, 38.74), a load's page says how far the drop-off is and in which
  direction, such as "222 km south of you", from the last position the phone
  had. This works offline. With signal, a small map shows you and the
  drop-off.
- **Delivering away from the drop-off.** Open **Record delivery** while far
  from the destination: a warning says how far away you are before anyone
  signs. **Go back**, or pick why you're delivering here (the recipient met the
  truck elsewhere, the destination is wrong, the phone's location is off, or
  other). The button reads **Deliver here anyway**, and "Other" needs a note.
  On the dashboard, the delivery shows **Away from the destination because …**
  and the location alert gives the driver's reason. The tolerance is 500 m
  for a destination with an exact location and 15 km for a city.
- **Suggestions.** The recipient's name offers the customer and the people
  who signed for this customer in the last year. **Quick notes** under Notes
  add "Left at reception" and similar lines. **Message the office** has quick
  messages such as "On my way".
- **Weak signal.** Add latency of over 10 seconds in DevTools (a custom
  network profile) and tap the status pill. It shows **Weak signal** after
  the slow sync, and while the phone says it's online but syncs keep
  failing. On a weak signal, deliveries go one at a time.
- **Battery saver.** In the menu, **Battery saver** records a position every
  15 minutes and syncs every 5. It turns on by itself below 20% battery when
  the phone isn't charging, and the menu then says so. The driver's page on
  the dashboard shows "Saving battery: a position every 15 minutes", and the
  map popup says the same. Parked for 10 minutes, the app stops the GPS and
  checks every 5 minutes (15 when saving battery) until the truck moves.
- **Keep the screen on while charging** holds the screen on during long
  drives, only while the phone is plugged in and the app is open.
- **Storage.** The menu shows how much the app holds and whether the browser
  protects it from automatic clean-up (an installed app usually is). The
  list warns when the phone is nearly full: under 50 MB left, or more than
  80% of the browser's allowance used. **Clear saved data** asks first, needs
  signal, and downloads the current work again. Updates not yet sent stay.
- **Settings follow the driver.** Choose Amharic and battery saver on one
  phone, then sign in on another. The second phone starts in the browser's
  language and switches to the driver's choices after the first sync. Signing
  in on the second phone signs the first one out, and the first phone forgets
  its shipments and messages.
- **Help** in the menu calls the office on the company's contact phone.

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
what the driver ran into.

A proof of delivery is never discarded. Record one offline, then cancel the
shipment, or give it to another driver, before the phone reconnects:
- The phone says the delivery record was kept for the office to review.
- The shipment page shows **Delivery to review** with the signature, photos
  and the reason it was held, and a critical **Delivery needs review** alert.
- **Accept delivery** marks the shipment delivered at the time of the proof
  and credits the driver who made it. The other driver's phone drops the
  shipment, and the customer gets the "delivered" SMS.
- **Dismiss** leaves the shipment as it is. The proof stays on the page,
  marked dismissed.

Cancelling a load that is already on the road offers reasons for a trip that
ended without delivery: **Recipient refused delivery**, **Returned to
origin**, **Cargo lost or damaged**. Reports count cancellations by reason.

## 6. Drivers without a smartphone

Settings → Text messages → **Try a driver's SMS report**, from a driver's
number, or from a second terminal (from `backend/`):
`go run ./cmd/server sms-in 0912345601 "ET 7K3M 1"`.

| Text | Effect |
|---|---|
| `ET 7K3M 1` | departed (in transit) |
| `ET 7K3M 2 traffic at Awash` | delayed, with the note |
| `ET 7K3M 3` | arrived |
| `ET 7K3M 5 flat tyre` | problem reported, with an alert |
| `help` or anything unparseable | a reply explaining the format |

`7K3M` stands for the last four characters of the tracking number. A text
from an unknown number changes nothing and gets no reply. The simulator and
`sms-in` say what happened to each message: recorded, refused and why,
answered with help, or ignored because the number isn't an active driver's
or isn't a phone number at all.

## 7. The customer

1. Open the tracking link from an SMS, or enter the number at `/track`. The
   page shows the status steps, due time, a coarse last-known position ("45 km
   from Adama") and public history. It never shows phone numbers or the
   driver's name.
2. `/track/phone`: enter the customer's phone, read the 6-digit code in the
   server terminal, and see all of their shipments.
3. Open the confirmation link from the delivery SMS. Confirm, or report a
   problem with a note. A reported problem raises an alert for the office.

## 8. Running the business

### Overview

- Five figures: active shipments, drivers on duty out of all drivers, loads
  delayed or with a problem, loads waiting for a driver, and deliveries today.
- The live map shows each driver's last position (green = online, amber =
  quiet, red = offline, a red ring = carrying a load that is delayed or has a
  problem), each destination, and a dashed line from origin to destination
  for every active load. Filter it by status, driver or region: the count
  beside the filters says how many loads match. Click a driver to see their
  loads and highlight their routes. The layers button switches to satellite
  imagery.
- **Needs attention** lists open alerts. **Recent activity** shows the last
  20 events.

Check: report a delay by SMS from a second terminal (section 6). Within a few
seconds, without a reload, the figures, alerts and activity change. Nothing
moves while you are typing or have a menu open; the page catches up after.

### Shipments

- Tabs (unassigned, active, needs attention, delivered, cancelled), search,
  filters and CSV export. Matches are highlighted. A phone number is found
  however it is written: `0922 334 455`, `+251922334455` or `922334455`.
- Click a column heading to sort by it, and again to reverse. **Columns**
  hides columns, and this browser remembers the choice.
- A driver whose app hasn't synced for 30 minutes while carrying a load is
  marked in red ("synced 2 hours ago").
- Under **Search and filter**, **Save this view as** keeps the tab, search
  and filters as a chip above the list. × removes it. Each person has their
  own views.
- On **Unassigned**, tick shipments (the box in the heading ticks them all),
  choose a driver and **Assign**. Each shipment is assigned on its own: if
  the driver reaches their active-load limit, the message lists the ones that
  were not given to them and why, and the rest are assigned.

Check: change the destination of an assigned shipment. The driver gets an SMS
("now goes to Hawassa"), and the app shows it.

A shipment's page has a **Location history** with the newest 200 positions:
time, coordinates, accuracy, speed and battery. **Download every position as
CSV** has all of them.

### Drivers

- Each driver is **On duty** or **Off duty**. On duty means carrying a load
  that has left, or having the app open and sharing location in the last half
  hour. The overview counts the same way.
- A driver's page shows the phone's battery, app version and storage used.
  **Reset PIN** texts the new temporary PIN to the driver.
- **Message drivers on the road** writes once to every driver with a load
  assigned or under way, either typed or picked from the quick messages.
  Drivers whose app hasn't synced for 15 minutes get it by SMS as well.
  **Sent** lists earlier messages and how many went by SMS. Each shipment
  page offers the same quick messages.

### Alerts

The alert types are:
- late shipments;
- loads not collected;
- problems and delays;
- drivers quiet for four hours while carrying cargo;
- deliveries far from the destination;
- **Moving away from the destination**: a load in transit that has got more
  than 20 km further from its destination over the last hour. Testing this
  takes an hour of real positions, so `TestHeadingAwayFromTheDestination`
  covers it.

Each delivery also raises an information alert. Turn it off with **Settings →
Alert the office when a shipment is delivered**. Dismiss alerts one at a time,
or mark all read.

Under **Account → Alerts**:
- **Turn on notifications in this browser** makes critical alerts pop up as
  system notifications while Menged is in a background tab.
- **Email me these alerts** picks the alert types that are emailed to you as
  well, once your email address is confirmed. Each email links to the
  shipment.

Check: tick **Issue reported** and save. Then send `ET 7K3M 5 flat tyre` by
SMS: the email appears in the server terminal.

### Reports

- Choose a period or a preset, a driver, and a route by its towns. A town is
  found under its other names too, so `Nazret` finds Adama. The chart groups
  deliveries per day, week or month.
- Every table has a CSV link that follows the filters.
- **Print or save as PDF** prints the page in light colours, whatever the
  screen's theme.

### The rest

- **Settings**: company details, which SMS go out, the active-load limit per
  driver, and the delivered alert.
- **Audit log**: sign-ins, failed sign-ins and lockouts, credential resets,
  and every change to people, vehicles and settings, with before and after
  values.
- **Account**: your profile and password, and the browsers and phones you are
  signed in on ("Chrome on Android"), which you can sign out. A new email
  address must be confirmed again: a link goes to it, and reset links sent to
  the old address stop working.

### Help and your data

- **Help** holds questions to the Menged team. **Ask a question** with a
  subject and a message. The team is emailed (an `EMAIL to admin@menged.et`
  block in the terminal). Its answer shows on the question's page and is
  emailed to whoever asked. Reply to carry on, or **Close** the question when
  it's settled; writing again reopens it. **Ask for an offer** under
  Enterprise on the Billing page starts a question with the subject filled
  in.
- **Settings → Your data → Download all data** gives one ZIP file: shipments,
  their history, deliveries, people and vehicles as CSV files (Excel opens
  them with Amharic intact), and each delivery's signature and photos under
  `deliveries/<tracking number>/`. `README.txt` explains the files. GPS
  points aren't included: they are kept for 90 days, and a shipment's route
  downloads from its page. The download is recorded in the audit log.
- **Removing someone's details.** Deactivate a driver or team member, then
  choose **Remove details** on their page. They become "Former driver" or
  "Former staff member", and their phone, email, sign-in, phone data and the
  texts and emails sent to them are deleted. Shipments, deliveries and the
  audit log still show what they did, under the new name. It can't be
  undone, and it isn't offered for someone still active.
- **Old customer details go by themselves.** Once a delivered or cancelled
  shipment is older than `RETAIN_SHIPMENT_DAYS` (365 days by default, at
  least 30), the hourly cleanup removes the customer's name, phone and
  addresses, notes, the delivery's signature and photos, and the texts about
  it. Dates, cities, statuses, reasons, the driver and the cargo stay, so
  reports still add up. The shipment page says what was removed, and it can't
  be reopened or booked again. To try it, backdate a delivered shipment
  (replace the tracking number), then restart `make run`: the cleanup runs at
  start.

  ```bash
  docker compose exec postgres psql -U logistics -c "UPDATE shipments SET actual_delivery = NOW() - INTERVAL '400 days', status_changed_at = NOW() - INTERVAL '400 days' WHERE tracking_number = 'ET-…'"
  ```

## 9. Paying for the product

As the company's admin, open **Billing**. It shows the trial's days left, the
plans, this month's usage (drivers and texts against the plan, shipments,
deliveries) and the invoices.

Some steps need time to pass. Move the clock in the database from `backend/`
(replace the company name), and the background jobs, which run every minute,
do the rest:

```bash
docker compose exec postgres psql -U logistics -c "UPDATE tenants SET trial_ends_at = NOW() WHERE name = 'Your company'"
```

1. **Choose a plan.** Under **Choose a plan**, pick Monthly or Annual and
   **Choose Starter**. An invoice opens, due in 7 days. A notice explains that
   the period starts when the trial ends, so paying early doesn't cut the
   trial short.
2. **Pay by mobile money.** If your email isn't confirmed yet, the invoice
   asks you to confirm it first (receipts go there). **Pay with telebirr**
   takes you to telebirr's checkout, played by a page of this server
   (`/dev/payments/…`; a narrow window shows it as a phone would). Try
   **Cancel** first: you are back on the payment page, nothing is paid, and
   the invoice stays open. Pay again, and **Pay** on the checkout: the
   payment page says **Paid**, the invoice is marked paid, and a receipt
   email is printed. With CBE Birr the payer pays from their phone, quoting
   the invoice's reference: **Play the payer's phone** on the invoice opens a
   page playing CBE Birr with the merchant ID and reference filled in.
   **Next** shows what the reference owes, and **Pay** pays it.
3. **Pay by bank transfer.** The invoice lists the bank details
   (`BILLING_BANK_DETAILS`) and asks for the invoice number as the reference.
   The operator records the transfer (section 10).
4. **The trial ends.** Set `trial_ends_at` to now on a company still on the
   trial, without a paid plan. Within a minute, an email says the trial has
   ended (earlier ones went out 3 days and 1 day before). The banner says the
   account is read-only. **New shipment**, assigning an unassigned load, and
   adding a driver, vehicle or person are refused with an explanation, while
   a load already on the road still takes driver updates and a delivery.
   Choosing a plan and paying lifts it at once.
5. **Renewal.** On a paid company, end the period:
   `UPDATE tenants SET period_end = NOW() WHERE name = '…'`. Within a minute a
   renewal invoice is issued and emailed, and the period moves on. Make it
   overdue (`UPDATE invoices SET due_at = NOW() - INTERVAL '1 day' WHERE
   number = 'INV-…'`): the banner warns when the account becomes read-only.
   With `INTERVAL '8 days'` it is read-only until the invoice is paid.
6. **Upgrade and downgrade.** On Starter, **Upgrade to Business**: an invoice
   for the difference over the rest of the period. Business applies, with its
   driver limit, as soon as it is paid. **Move to Starter** from Business
   waits for the end of the period, and is refused while more drivers are
   active than Starter allows. **Keep things as they are** cancels a pending
   change. While a renewal or usage invoice is unpaid, plan buttons say
   which invoice to pay first.
7. **Limits.** At 80% of the plan's driver places, an alert says so, once
   per billing period; at 80% of the month's texts, once a month. Texts are
   never blocked: those beyond the allowance appear on a usage invoice early
   the next month, at ETB 0.50 each, with the month's count.

Check:
- **Print** on an invoice gives a one-page document with the company's TIN
  when it has one, and VAT at 15%.
- On a phone, the invoice table fits the screen: quantities move under each
  description, and totals line up under the amounts.
- Paying at the same checkout twice, or a notification with a wrong
  signature or amount, changes nothing (the integration tests cover the
  signed notification endpoint).

## 10. The operator

Sign in as `admin@menged.et`. **Companies** lists every company with plan,
drivers, activity and billing state. Company pages such as `/app/shipments`
send the operator back to this list. A company's row opens its page: usage,
plan, invoices and questions, and these actions:
- **Extend trial** by a number of days.
- **Offer Enterprise**: a monthly price, driver limit and text allowance. The
  company is emailed the terms and chooses Enterprise on its billing page.
- **Suspend** the company: its users are signed out at once and can't sign
  back in until it is restored.
- **Delete** the company, by typing its name exactly. Its people, vehicles,
  shipments, deliveries, photos and audit log go for good, and its users are
  signed out. Its invoices stay, without the company, and open ones are
  cancelled. A payment that arrives afterwards is set aside as a refund (see
  below). The platform's audit log keeps one entry recording the deletion.

**Support** lists every company's questions, those waiting for an answer
first; the menu shows how many are waiting. Answering emails whoever asked.

**Audit log** searches every company's log by company, action, date and IP
address. Clicking an address shows everything done from it.

**System health** shows the database's response time and schema version, each
background job's last run, how long it took and its last error, the SMS and
email queues, live dashboard connections and free disk space. A job that
hasn't run for three of its intervals shows **Not running**.

**Billing** shows monthly recurring revenue, money collected this month,
unpaid and overdue invoices, and recent payments. On an unpaid invoice:
- **Mark paid** with the bank's reference records a transfer. The company is
  emailed a receipt, and a read-only account is lifted at once.
- **Cancel invoice** with a reason, which is emailed to the company.

As the company, start a telebirr payment on an invoice but leave the
checkout open without paying. As the operator, mark the same invoice paid.
Then **Pay** on the checkout. The late payment is not applied to anything:
it appears under **Refunds to send**, and its payment page tells the company
it will be refunded. Record the refund with the bank's reference, and the
company is emailed. (Two checkouts paid for one invoice end the same way.)

## 11. Security checks

- Five wrong passwords lock the account for 15 minutes. Further attempts from
  the same address are rate-limited.
- Deactivating a driver or staff member ends their sessions at once. The
  driver app returns to the sign-in screen on its next sync, and the
  shipments and messages on the phone (customers' names and numbers) are
  erased. Updates it hadn't sent yet are kept for that driver's next sign-in.
- One company's IDs in another company's URLs return 404.
- Forms posted from another site are refused (cross-origin protection).
- `/metrics` returns 404 unless `METRICS_TOKEN` is set and sent as a bearer
  token. With it, `curl -H "Authorization: Bearer $METRICS_TOKEN"
  localhost:8080/metrics` lists the `menged_*` metrics, with requests labelled
  by route pattern (`route="/app/shipments/{id}"`) and no IDs.
- The audit log and shipment histories can't be changed or deleted, even in
  the database:

  ```bash
  docker compose exec postgres psql -U logistics -c "DELETE FROM audit_logs"
  ```

  fails with "audit_logs is append-only". Only deleting a company removes
  them.

## 12. Through the provider sandbox

The same flows with the app's real SMS, telebirr and CBE Birr adapters,
against the provider sandbox instead of the stand-ins. Stop `make run`, and
in two terminals from `backend/`:

```bash
make sandbox       # http://localhost:8091
make run-sandbox   # http://localhost:8080, its providers pointed at the sandbox
```

Give both the same `PORT` to move the app. Email is still printed in the
app's terminal.

1. **The sign-up code.** Sign up a new company. The code isn't printed: it
   arrives on the sandbox's phone. Open the sandbox, open the phone by its
   number, read the code and finish signing up. Under the text, *Delivered ·
   Report: Taken by Menged*: the gateway's delivery report reached the app,
   and Settings → Text messages shows the text **Delivered**.
2. **A driver's text.** Add a driver and book a shipment for them. Their
   phone in the sandbox shows the assignment, ending "text ET <ref> 1 to
   6060". Send `ET <ref> 1` from the box under the thread: the text says what
   the app made of it, the shipment is In transit, and the confirmation
   arrives on the driver's phone.
3. **telebirr.** Choose a plan and **Pay with telebirr**. The browser goes to
   telebirr's checkout on the sandbox, with the amount, the invoice and the
   merchant. Try a wrong PIN first, then pay with any number and PIN `1234`.
   The checkout sends you back to the payment page, already paid, the
   receipt is emailed, and the payer's phone has telebirr's receipt from 127.
4. **CBE Birr.** On another company's invoice, follow **Pay from a sandbox
   phone**: the phone opens with the merchant ID and reference filled in.
   **Next** asks the app what the reference owes; enter PIN `1234` and
   **Pay**. The invoice is paid, and the phone has CBE Birr's receipt. The
   same reference again is turned away: the invoice is paid.
5. **Faults.** On **Faults**, turn one on at a time:
   - *Notifications are dropped*: pay with telebirr. The checkout sends you
     back, and the payment page asks telebirr and finds it paid. The dropped
     notification waits under **Held back** on the sandbox's home page, and
     **Send now** changes nothing in the app.
   - *Notifications are forged*: the app answers the notification 401 (see
     **Log**), and still learns the outcome by asking telebirr.
   - *The gateway is down*: book a shipment. Settings → Text messages shows
     the driver's text waiting. Turn the fault off: it goes at the next try,
     1, 5 and 15 minutes after the first.
   - *Barred numbers*: enter a driver's number and book for them: the text
     fails at once, without retrying. *Unreachable numbers*: it is sent, then
     reported **Not delivered**.
   - *Confirmations are dropped*: pay by CBE Birr. The money leaves the
     wallet but the invoice stays open until the held confirmation is sent.
6. **Log.** Every call in both directions, with headers and bodies, secrets
   masked.

Check:
- **System health** (as the operator) names the providers **Sandbox**.
- The sandbox's pages work on a phone and in dark mode.
- `docker compose --profile sandbox up sandbox` runs the sandbox in a
  container instead of `make sandbox`, for the app on the host.
