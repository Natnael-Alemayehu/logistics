-- +goose Up
-- +goose StatementBegin

-- The Growth plan is called Business now.
UPDATE tenants SET plan = 'business' WHERE plan = 'growth';

-- ---------------------------------------------------------------------------
-- A company's subscription. The current period is the one paid for; a
-- downgrade or a change of billing cycle waits in pending_* until it ends.
-- The platform operator offers a company Enterprise terms (price and
-- limits), which the company then takes up like any other plan. Prices and
-- amounts are in santim, hundredths of a birr.
--
-- The text allowance is per calendar month. sms_month_quota is the allowance
-- for the month in progress: the highest the company had during it, so a
-- downgrade mid-month doesn't turn texts already sent into overage.
-- sms_billed_month is the first day of the month being metered: once that
-- month is over, texts beyond its allowance are invoiced and the next begins.
-- ---------------------------------------------------------------------------
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS billing_cycle TEXT NOT NULL DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'annual')),
    ADD COLUMN IF NOT EXISTS period_start TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS period_end TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS pending_plan TEXT,
    ADD COLUMN IF NOT EXISTS pending_cycle TEXT CHECK (pending_cycle IN ('monthly', 'annual')),
    ADD COLUMN IF NOT EXISTS enterprise_price_santim BIGINT CHECK (enterprise_price_santim >= 0),
    ADD COLUMN IF NOT EXISTS enterprise_max_drivers INTEGER,
    ADD COLUMN IF NOT EXISTS enterprise_sms_quota INTEGER,
    ADD COLUMN IF NOT EXISTS subscribed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sms_quota INTEGER NOT NULL DEFAULT 500,
    ADD COLUMN IF NOT EXISTS sms_month_quota INTEGER NOT NULL DEFAULT 500,
    ADD COLUMN IF NOT EXISTS sms_billed_month DATE,
    ADD COLUMN IF NOT EXISTS tin TEXT;

UPDATE tenants SET sms_quota = CASE plan WHEN 'business' THEN 3000 WHEN 'enterprise' THEN 10000 ELSE 500 END;
UPDATE tenants SET sms_month_quota = sms_quota,
    sms_billed_month = date_trunc('month', NOW() AT TIME ZONE 'Africa/Addis_Ababa')::date;
-- A trial has the Starter plan's limits.
UPDATE tenants SET max_drivers = 10 WHERE plan = 'trial' AND max_drivers < 10;
-- Companies already on a paid plan start their first billed period now.
UPDATE tenants SET period_start = NOW(), period_end = NOW() + INTERVAL '1 month', subscribed_at = NOW()
WHERE plan IN ('starter', 'business') AND period_end IS NULL;

-- ---------------------------------------------------------------------------
-- Invoices are numbered INV-<year>-<n>, without gaps: the counter moves in
-- the transaction that issues the invoice. An invoice keeps the customer's
-- details as they were, and outlives the company.
--
-- Kinds: subscription (a first plan), upgrade (the prorated difference for
-- the rest of the period), renewal (the next period) and usage (texts beyond
-- the allowance). Unpaid renewal and usage invoices are debts and lead to
-- read-only mode; unpaid subscription and upgrade invoices just lapse.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invoice_counters (
    year INTEGER PRIMARY KEY,
    last INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    number TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('subscription', 'upgrade', 'renewal', 'usage')),
    plan TEXT NOT NULL,
    billing_cycle TEXT NOT NULL CHECK (billing_cycle IN ('monthly', 'annual')),
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    lines JSONB NOT NULL,
    -- What the company used when the invoice was issued, kept for disputes
    -- after the detail (such as the SMS log) has been deleted.
    usage JSONB,
    subtotal_santim BIGINT NOT NULL,
    vat_santim BIGINT NOT NULL,
    total_santim BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'paid', 'void')),
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    customer_name TEXT NOT NULL,
    customer_tin TEXT,
    customer_city TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_open ON invoices(tenant_id, due_at) WHERE status = 'open';

-- ---------------------------------------------------------------------------
-- Attempts to pay an invoice. The reference is ours and unique per attempt;
-- a provider's notification is matched to it, and repeats change nothing.
-- A bank transfer is recorded by the platform operator. Money that arrives
-- for an invoice already paid or cancelled is kept on record with
-- refund_due set, until the operator has sent it back.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    invoice_id UUID NOT NULL REFERENCES invoices(id),
    provider TEXT NOT NULL,
    reference TEXT NOT NULL UNIQUE,
    provider_ref TEXT,
    payer_phone TEXT,
    amount_santim BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed', 'expired', 'refunded')),
    raw JSONB,
    note TEXT,
    refund_due BOOLEAN NOT NULL DEFAULT false,
    refunded_at TIMESTAMPTZ,
    refund_reference TEXT,
    recorded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_payments_invoice ON payments(invoice_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_paid ON payments(completed_at) WHERE status = 'paid';
CREATE INDEX IF NOT EXISTS idx_payments_pending ON payments(created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_payments_refund_due ON payments(created_at) WHERE refund_due;

-- ---------------------------------------------------------------------------
-- Billing emails sent once per situation: a trial about to end, an invoice
-- overdue, a company gone read-only.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS billing_notices (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    episode TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, kind, episode)
);

-- ---------------------------------------------------------------------------
-- Alerts about the company as a whole have neither a shipment nor a driver.
-- Unique indexes treat NULLs as distinct, so these were never deduplicated:
-- the company stands in as the subject.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS idx_alerts_open_subject;
CREATE UNIQUE INDEX idx_alerts_open_subject
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id))
    WHERE acknowledged_at IS NULL;
DROP INDEX IF EXISTS idx_alerts_episode;
CREATE UNIQUE INDEX idx_alerts_episode
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id), episode)
    WHERE episode IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_alerts_episode;
CREATE UNIQUE INDEX idx_alerts_episode
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id), episode)
    WHERE episode IS NOT NULL;
DROP INDEX IF EXISTS idx_alerts_open_subject;
CREATE UNIQUE INDEX idx_alerts_open_subject
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id))
    WHERE acknowledged_at IS NULL;
DROP TABLE IF EXISTS billing_notices;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS invoice_counters;
ALTER TABLE tenants
    DROP COLUMN IF EXISTS billing_cycle,
    DROP COLUMN IF EXISTS period_start,
    DROP COLUMN IF EXISTS period_end,
    DROP COLUMN IF EXISTS pending_plan,
    DROP COLUMN IF EXISTS pending_cycle,
    DROP COLUMN IF EXISTS enterprise_price_santim,
    DROP COLUMN IF EXISTS enterprise_max_drivers,
    DROP COLUMN IF EXISTS enterprise_sms_quota,
    DROP COLUMN IF EXISTS subscribed_at,
    DROP COLUMN IF EXISTS sms_quota,
    DROP COLUMN IF EXISTS sms_month_quota,
    DROP COLUMN IF EXISTS sms_billed_month,
    DROP COLUMN IF EXISTS tin;
UPDATE tenants SET plan = 'growth' WHERE plan = 'business';
-- +goose StatementEnd
