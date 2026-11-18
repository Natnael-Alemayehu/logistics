-- +goose Up
-- What a company is alerted about, and when. One row per company, written
-- the first time its rules are changed; until then the defaults below
-- apply, which are what the checks did before the rules could be set.
CREATE TABLE IF NOT EXISTS alert_rules (
    tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    -- A driver reports a delay or a problem.
    problems_on boolean NOT NULL DEFAULT true,
    -- A load on the road is this long past its ETA.
    late_on boolean NOT NULL DEFAULT true,
    late_minutes integer NOT NULL DEFAULT 0 CHECK (late_minutes BETWEEN 0 AND 1440),
    -- A driver carrying a load hasn't been heard from for this long.
    offline_on boolean NOT NULL DEFAULT true,
    offline_minutes integer NOT NULL DEFAULT 240 CHECK (offline_minutes BETWEEN 15 AND 1440),
    -- A truck on the road has stood still this long outside a town.
    stop_on boolean NOT NULL DEFAULT true,
    stop_minutes integer NOT NULL DEFAULT 45 CHECK (stop_minutes BETWEEN 10 AND 720),
    -- A truck is this much further from the destination than an hour before.
    route_on boolean NOT NULL DEFAULT true,
    route_km integer NOT NULL DEFAULT 20 CHECK (route_km BETWEEN 2 AND 200),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);

-- What was done about an alert, written by whoever resolved it.
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS resolution_note text
    CONSTRAINT alerts_resolution_note_check CHECK (char_length(resolution_note) <= 300);

-- The alert types texted to a person, beside those emailed (alert_emails).
ALTER TABLE users ADD COLUMN IF NOT EXISTS alert_sms text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS alert_sms;
ALTER TABLE alerts DROP COLUMN IF EXISTS resolution_note;
DROP TABLE IF EXISTS alert_rules;
