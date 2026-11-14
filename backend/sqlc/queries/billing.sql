-- Subscriptions, invoices and payments (FR-043 to FR-045).

-- name: GetTenantForUpdate :one
SELECT * FROM tenants WHERE id = $1 FOR UPDATE;

-- name: SaveSubscription :exec
-- Writes a company's subscription as the billing service decided it. The
-- month's text allowance only ever rises until the month ends.
UPDATE tenants
SET plan = sqlc.arg(plan),
    billing_cycle = sqlc.arg(billing_cycle),
    max_drivers = sqlc.arg(max_drivers),
    sms_quota = sqlc.arg(sms_quota),
    sms_month_quota = GREATEST(sms_month_quota, sqlc.arg(sms_quota)::int),
    period_start = sqlc.narg(period_start),
    period_end = sqlc.narg(period_end),
    subscribed_at = sqlc.narg(subscribed_at),
    pending_plan = sqlc.narg(pending_plan),
    pending_cycle = sqlc.narg(pending_cycle),
    billing_anchor_day = sqlc.narg(billing_anchor_day),
    renewal_cancelled_at = sqlc.narg(renewal_cancelled_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: SetEnterpriseOffer :exec
UPDATE tenants
SET enterprise_price_santim = sqlc.narg(price_santim),
    enterprise_max_drivers = sqlc.narg(max_drivers),
    enterprise_sms_quota = sqlc.narg(sms_quota),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: SetTrialEnd :exec
UPDATE tenants SET trial_ends_at = $2, updated_at = NOW() WHERE id = $1 AND plan = 'trial';

-- name: SetSMSBilledMonth :exec
-- The month is settled: the next one starts with the current allowance.
UPDATE tenants SET sms_billed_month = $2, sms_month_quota = sms_quota WHERE id = $1;

-- name: NextInvoiceNumber :one
-- Moves the year's counter within the caller's transaction, so numbers have
-- no gaps.
INSERT INTO invoice_counters (year, last) VALUES ($1, 1)
ON CONFLICT (year) DO UPDATE SET last = invoice_counters.last + 1
RETURNING last;

-- name: CreateInvoice :one
INSERT INTO invoices (
    tenant_id, number, kind, plan, billing_cycle, period_start, period_end,
    lines, usage, subtotal_santim, vat_santim, total_santim, issued_at, due_at,
    customer_name, customer_tin, customer_city, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(number), sqlc.arg(kind), sqlc.arg(plan), sqlc.arg(billing_cycle),
    sqlc.narg(period_start), sqlc.narg(period_end),
    sqlc.arg(lines), sqlc.narg(usage), sqlc.arg(subtotal_santim), sqlc.arg(vat_santim), sqlc.arg(total_santim),
    sqlc.arg(issued_at), sqlc.arg(due_at),
    sqlc.arg(customer_name), sqlc.narg(customer_tin), sqlc.narg(customer_city), sqlc.narg(created_by)
) RETURNING *;

-- name: GetInvoice :one
SELECT * FROM invoices WHERE id = $1 AND tenant_id = $2;

-- name: LockInvoice :one
SELECT * FROM invoices WHERE id = $1 AND tenant_id = $2 FOR UPDATE;

-- name: GetInvoiceByNumber :one
-- A payer quotes the number to a provider, which asks us about it. Numbers
-- are the platform's and unique, so this finds one company's invoice.
SELECT * FROM invoices WHERE number = $1;

-- name: LockInvoiceByNumber :one
SELECT * FROM invoices WHERE number = $1 FOR UPDATE;

-- name: ListInvoices :many
SELECT * FROM invoices WHERE tenant_id = $1 ORDER BY issued_at DESC, number DESC LIMIT $2;

-- name: GetOldestDebt :one
-- The oldest unpaid renewal or usage invoice: what read-only mode hangs on.
SELECT * FROM invoices
WHERE tenant_id = $1 AND status = 'open' AND kind IN ('renewal', 'usage')
ORDER BY due_at LIMIT 1;

-- name: GetOpenRenewal :one
-- The renewal invoice waiting for payment, if any. There is at most one.
SELECT * FROM invoices
WHERE tenant_id = $1 AND status = 'open' AND kind = 'renewal'
ORDER BY issued_at DESC LIMIT 1;

-- name: CountOpenRenewals :one
SELECT COUNT(*) FROM invoices WHERE tenant_id = $1 AND status = 'open' AND kind = 'renewal';

-- name: GetOpenPlanInvoice :one
-- The plan choice waiting for payment, if any. There is at most one.
SELECT * FROM invoices
WHERE tenant_id = $1 AND status = 'open' AND kind IN ('subscription', 'upgrade')
ORDER BY issued_at DESC LIMIT 1;

-- name: VoidOpenPlanInvoices :exec
-- A new plan choice replaces one not yet paid for.
UPDATE invoices SET status = 'void', voided_at = NOW()
WHERE tenant_id = $1 AND status = 'open' AND kind IN ('subscription', 'upgrade');

-- name: VoidInvoice :execrows
UPDATE invoices SET status = 'void', voided_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND status = 'open';

-- name: VoidLapsedPlanInvoices :many
-- For the billing job, across companies: a plan invoice left unpaid past
-- its due date lapses, since its price was worked out for that moment.
UPDATE invoices SET status = 'void', voided_at = NOW()
WHERE status = 'open' AND kind IN ('subscription', 'upgrade') AND due_at < $1
RETURNING id, tenant_id, number;

-- name: MarkInvoicePaid :exec
-- A plan's period runs from when it is paid for, so a subscription invoice
-- gets its dates on payment, and a renewal paid after read-only mode began
-- has them moved to the payment.
UPDATE invoices
SET status = 'paid', paid_at = sqlc.arg(paid_at),
    period_start = COALESCE(sqlc.narg(period_start), period_start),
    period_end = COALESCE(sqlc.narg(period_end), period_end)
WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id);

-- name: CreatePayment :one
INSERT INTO payments (
    tenant_id, invoice_id, provider, reference, provider_ref, payer_phone, amount_santim,
    status, raw, note, recorded_by, completed_at
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(invoice_id), sqlc.arg(provider), sqlc.arg(reference),
    sqlc.narg(provider_ref), sqlc.narg(payer_phone), sqlc.arg(amount_santim), sqlc.arg(status),
    sqlc.narg(raw), sqlc.narg(note), sqlc.narg(recorded_by), sqlc.narg(completed_at)
) RETURNING *;

-- name: SetPaymentProviderRef :exec
UPDATE payments SET provider_ref = $2 WHERE id = $1;

-- name: LockPaymentByReference :one
-- A provider's notification carries only our reference. References are
-- random and unique, so this finds one company's payment.
SELECT * FROM payments WHERE reference = $1 FOR UPDATE;

-- name: GetPayment :one
SELECT p.*, i.number AS invoice_number
FROM payments p JOIN invoices i ON i.id = p.invoice_id
WHERE p.id = $1 AND p.tenant_id = $2;

-- name: FinishPayment :exec
UPDATE payments
SET status = sqlc.arg(status),
    provider_ref = COALESCE(sqlc.narg(provider_ref), provider_ref),
    raw = COALESCE(sqlc.narg(raw), raw),
    note = COALESCE(sqlc.narg(note), note),
    completed_at = NOW()
WHERE id = sqlc.arg(id);

-- name: FlagRefundDue :exec
UPDATE payments SET refund_due = true, note = $2 WHERE id = $1;

-- name: LockPayment :one
SELECT p.*, i.number AS invoice_number, i.total_santim AS invoice_total
FROM payments p JOIN invoices i ON i.id = p.invoice_id
WHERE p.id = $1
FOR UPDATE OF p;

-- name: MarkPaymentRefunded :exec
UPDATE payments
SET status = 'refunded', refund_due = false, refunded_at = NOW(),
    refund_reference = sqlc.arg(refund_reference), note = sqlc.arg(note)
WHERE id = sqlc.arg(id);

-- name: ClaimPaymentCheck :one
-- Notes that the provider is being asked how a payment ended, unless it was
-- asked since checked_before. No row means another check is under way or
-- was recent.
UPDATE payments SET checked_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('pending', 'expired')
  AND (checked_at IS NULL OR checked_at < sqlc.arg(checked_before))
RETURNING *;

-- name: ListPaymentsToCheck :many
-- Payments a provider may have finished without its notification reaching
-- us: open a minute or more, a few hours old at most, and not asked about
-- recently. Across companies, for the billing job.
SELECT * FROM payments
WHERE status IN ('pending', 'expired') AND provider = ANY(sqlc.arg(providers)::text[])
  AND created_at < sqlc.arg(started_before) AND created_at > sqlc.arg(started_after)
  AND (checked_at IS NULL OR checked_at < sqlc.arg(checked_before))
ORDER BY created_at
LIMIT 50;

-- name: ExpirePendingPayments :exec
-- A prompt nobody answered. A late approval is still honoured.
UPDATE payments SET status = 'expired', completed_at = NOW()
WHERE status = 'pending' AND created_at < $1;

-- name: ListPaymentsByInvoice :many
SELECT * FROM payments WHERE invoice_id = $1 AND tenant_id = $2 ORDER BY created_at DESC;

-- name: AddBillingNotice :execrows
INSERT INTO billing_notices (tenant_id, kind, episode) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: ListBillingContacts :many
-- The company's active admins with an email address: invoices and billing
-- notices go to those who confirmed theirs.
SELECT full_name, email::text AS email, (email_verified_at IS NOT NULL)::bool AS verified FROM users
WHERE tenant_id = $1 AND role = 'admin' AND is_active AND email IS NOT NULL
ORDER BY created_at;

-- name: CountSMSSentBetween :one
-- In SMS parts: a long text counts as several.
SELECT COALESCE(SUM(segments), 0)::bigint FROM sms_messages
WHERE tenant_id = $1 AND direction = 'outbound' AND status = 'sent'
  AND sent_at >= sqlc.arg(since) AND sent_at < sqlc.arg(until);

-- ---------------------------------------------------------------------------
-- The billing job looks across companies.
-- ---------------------------------------------------------------------------

-- name: ListTenantsToRenew :many
-- Paid periods that have ended, unless the last renewal is still unpaid or
-- the company turned renewal off.
SELECT * FROM tenants t
WHERE t.is_active AND t.plan <> 'trial' AND t.period_end IS NOT NULL AND t.period_end <= $1
  AND t.renewal_cancelled_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM invoices i WHERE i.tenant_id = t.id AND i.status = 'open' AND i.kind = 'renewal');

-- name: ListTenantsForUsageBilling :many
-- Companies with a finished month whose texts are not yet settled.
SELECT * FROM tenants WHERE sms_billed_month IS NULL OR sms_billed_month < $1;

-- name: ListTrialsEnding :many
-- Trials ending before $1 whose end hasn't been announced yet. The episode
-- is the end time, so an extended trial is announced again.
SELECT * FROM tenants t
WHERE t.is_active AND t.plan = 'trial' AND t.trial_ends_at IS NOT NULL AND t.trial_ends_at <= $1
  AND NOT EXISTS (
      SELECT 1 FROM billing_notices n
      WHERE n.tenant_id = t.id AND n.kind = 'trial_ended'
        AND n.episode = floor(extract(epoch FROM t.trial_ends_at))::bigint::text
  );

-- name: ListOverdueDebts :many
-- Unpaid renewal and usage invoices past their due date, until the company
-- has been told it is read-only.
SELECT i.* FROM invoices i
JOIN tenants t ON t.id = i.tenant_id
WHERE t.is_active AND i.status = 'open' AND i.kind IN ('renewal', 'usage') AND i.due_at <= $1
  AND NOT EXISTS (
      SELECT 1 FROM billing_notices n
      WHERE n.tenant_id = i.tenant_id AND n.kind = 'read_only' AND n.episode = i.number
  );

-- name: ListTenantsNearLimits :many
-- Companies using 80% or more of their driver places or of the month's
-- texts.
WITH usage AS (
    SELECT t.id, t.plan, t.max_drivers, t.sms_month_quota, t.period_start, t.trial_ends_at,
           (SELECT COUNT(*) FROM users u WHERE u.tenant_id = t.id AND u.role = 'driver' AND u.is_active)::bigint AS active_drivers,
           (SELECT COALESCE(SUM(m.segments), 0) FROM sms_messages m
            WHERE m.tenant_id = t.id AND m.direction = 'outbound' AND m.status = 'sent'
              -- Texts from before a subscription started were the trial's.
              AND m.sent_at >= GREATEST(sqlc.arg(month_start)::timestamptz, t.subscribed_at))::bigint AS sms_sent
    FROM tenants t
    WHERE t.is_active
)
SELECT * FROM usage
WHERE (max_drivers > 0 AND active_drivers * 5 >= max_drivers * 4)
   OR (sms_month_quota > 0 AND sms_sent * 5 >= sms_month_quota * 4);

-- ---------------------------------------------------------------------------
-- The platform operator's view.
-- ---------------------------------------------------------------------------

-- name: GetInvoiceByID :one
SELECT i.*, COALESCE(t.name, i.customer_name)::text AS tenant_name
FROM invoices i LEFT JOIN tenants t ON t.id = i.tenant_id
WHERE i.id = $1;

-- name: GetPaymentByReference :one
-- What the stand-in payment prompt shows: the reference is the secret.
SELECT p.*, i.number AS invoice_number, COALESCE(t.name, i.customer_name)::text AS tenant_name
FROM payments p
JOIN invoices i ON i.id = p.invoice_id
LEFT JOIN tenants t ON t.id = p.tenant_id
WHERE p.reference = $1;

-- name: ListAllInvoices :many
SELECT i.*, COALESCE(t.name, i.customer_name)::text AS tenant_name
FROM invoices i LEFT JOIN tenants t ON t.id = i.tenant_id
WHERE (sqlc.arg(status)::text = '' OR i.status = sqlc.arg(status)::text)
ORDER BY i.issued_at DESC
LIMIT sqlc.arg(max_rows);

-- name: ListRecentPayments :many
SELECT p.*, i.number AS invoice_number, COALESCE(t.name, i.customer_name)::text AS tenant_name
FROM payments p
JOIN invoices i ON i.id = p.invoice_id
LEFT JOIN tenants t ON t.id = p.tenant_id
ORDER BY p.created_at DESC
LIMIT $1;

-- name: ListRefundsDue :many
SELECT p.*, i.number AS invoice_number, COALESCE(t.name, i.customer_name)::text AS tenant_name
FROM payments p
JOIN invoices i ON i.id = p.invoice_id
LEFT JOIN tenants t ON t.id = p.tenant_id
WHERE p.refund_due
ORDER BY p.created_at;

-- name: SumPaidSince :one
SELECT COALESCE(SUM(amount_santim), 0)::bigint FROM payments WHERE status = 'paid' AND completed_at >= $1;
