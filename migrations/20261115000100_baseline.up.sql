-- The whole schema, in one migration.
--
-- It replaces the 21 migrations that built the schema between February and
-- November 2026, which `git log -- migrations/` still shows. It was made
-- with pg_dump --schema-only from a database those migrations had built,
-- then tidied: constraints moved into their tables, each table after the
-- tables it refers to, and types and checks spelled the usual way. A
-- database built from this file dumps exactly as that one did.
--
-- Nothing else was carried over. The old migrations' UPDATEs and DELETEs
-- only corrected rows that existed at the time, and a new database has
-- none.
--
-- Every statement can run again without changing anything, so a database
-- the old migrations built takes this one as done. The Down drops
-- everything, PostGIS included.
--
-- append_only() lets rows of the append-only tables go only inside purgeTx
-- (internal/service/data.go), which sets kiflet.purge for its transaction.
-- update_updated_at_column() keeps updated_at current.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION append_only() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF current_setting('kiflet.purge', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS tenants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    plan text DEFAULT 'starter' NOT NULL,
    max_drivers integer DEFAULT 5 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    contact_phone text,
    contact_email text,
    region_city text DEFAULT 'Addis Ababa' NOT NULL,
    max_active_per_driver integer DEFAULT 5 NOT NULL,
    sms_on_created boolean DEFAULT true NOT NULL,
    sms_on_dispatched boolean DEFAULT true NOT NULL,
    sms_on_arrived boolean DEFAULT true NOT NULL,
    sms_on_delivered boolean DEFAULT true NOT NULL,
    sms_on_cancelled boolean DEFAULT true NOT NULL,
    sms_on_delayed boolean DEFAULT false NOT NULL,
    sms_driver_on_assign boolean DEFAULT true NOT NULL,
    trial_ends_at TIMESTAMPTZ,
    billing_cycle text DEFAULT 'monthly' NOT NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    pending_plan text,
    pending_cycle text,
    enterprise_price_santim bigint,
    enterprise_max_drivers integer,
    enterprise_sms_quota integer,
    subscribed_at TIMESTAMPTZ,
    sms_quota integer DEFAULT 500 NOT NULL,
    sms_month_quota integer DEFAULT 500 NOT NULL,
    sms_billed_month date,
    tin text,
    alert_on_delivery boolean DEFAULT true NOT NULL,
    billing_anchor_day integer,
    renewal_cancelled_at TIMESTAMPTZ,
    CONSTRAINT tenants_pkey PRIMARY KEY (id),
    CONSTRAINT tenants_slug_key UNIQUE (slug),
    CONSTRAINT tenants_billing_anchor_day_check CHECK ((billing_anchor_day >= 1) AND (billing_anchor_day <= 31)),
    CONSTRAINT tenants_billing_cycle_check CHECK (billing_cycle IN ('monthly', 'annual')),
    CONSTRAINT tenants_enterprise_price_santim_check CHECK (enterprise_price_santim >= 0),
    CONSTRAINT tenants_pending_cycle_check CHECK (pending_cycle IN ('monthly', 'annual'))
);

CREATE TABLE IF NOT EXISTS vehicles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    plate_number text NOT NULL,
    vehicle_type text,
    is_active boolean DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    capacity_kg numeric(10,2),
    CONSTRAINT vehicles_pkey PRIMARY KEY (id),
    CONSTRAINT vehicles_tenant_id_plate_number_key UNIQUE (tenant_id, plate_number),
    CONSTRAINT vehicles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    role text NOT NULL,
    full_name text NOT NULL,
    phone text,
    email text,
    password_hash text,
    pin_hash text,
    is_active boolean DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    failed_login_attempts integer DEFAULT 0 NOT NULL,
    locked_until TIMESTAMPTZ,
    password_reset_required boolean DEFAULT false NOT NULL,
    vehicle_id uuid,
    last_login_at TIMESTAMPTZ,
    email_verified_at TIMESTAMPTZ,
    alert_emails text[] DEFAULT '{}'::text[] NOT NULL,
    language text,
    app_battery_saver boolean DEFAULT false NOT NULL,
    app_vibrate boolean DEFAULT true NOT NULL,
    anonymised_at TIMESTAMPTZ,
    alerts_seen_at TIMESTAMPTZ,
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_tenant_id_email_key UNIQUE (tenant_id, email),
    CONSTRAINT users_tenant_id_phone_key UNIQUE (tenant_id, phone),
    CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT users_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE SET NULL,
    CONSTRAINT users_language_check CHECK (language IN ('en', 'am')),
    CONSTRAINT users_role_check CHECK (role IN ('driver', 'dispatcher', 'fleet_manager', 'admin', 'platform_admin'))
);

CREATE TABLE IF NOT EXISTS shipments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    tracking_number text NOT NULL,
    origin_address text NOT NULL,
    destination_address text NOT NULL,
    customer_name text NOT NULL,
    customer_phone text NOT NULL,
    cargo_description text,
    cargo_weight numeric(10,2),
    cargo_value numeric(15,2),
    special_instructions text,
    driver_id uuid,
    vehicle_id uuid,
    status text DEFAULT 'pending' NOT NULL,
    status_note text,
    status_reason text,
    estimated_delivery TIMESTAMPTZ,
    actual_delivery TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    created_by uuid,
    status_changed_at TIMESTAMPTZ,
    priority text DEFAULT 'normal' NOT NULL,
    origin_city text,
    destination_city text,
    origin_lat double precision,
    origin_lng double precision,
    destination_lat double precision,
    destination_lng double precision,
    destination_precise boolean DEFAULT false NOT NULL,
    picked_up_at TIMESTAMPTZ,
    confirm_token text,
    customer_confirmed_at TIMESTAMPTZ,
    customer_feedback text,
    eta_manual boolean DEFAULT false NOT NULL,
    planned_pickup_at TIMESTAMPTZ,
    anonymised_at TIMESTAMPTZ,
    CONSTRAINT shipments_pkey PRIMARY KEY (id),
    CONSTRAINT shipments_confirm_token_key UNIQUE (confirm_token),
    CONSTRAINT shipments_tracking_number_key UNIQUE (tracking_number),
    CONSTRAINT shipments_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT shipments_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT shipments_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE SET NULL,
    CONSTRAINT shipments_priority_check CHECK (priority IN ('normal', 'high', 'urgent')),
    CONSTRAINT shipments_status_check CHECK (status IN ('pending', 'assigned', 'in_transit', 'delayed', 'arrived', 'delivered', 'issue', 'cancelled'))
);

CREATE TABLE IF NOT EXISTS alerts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid,
    driver_id uuid,
    alert_type text NOT NULL,
    severity text DEFAULT 'warning' NOT NULL,
    message text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    read_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by uuid,
    episode text,
    CONSTRAINT alerts_pkey PRIMARY KEY (id),
    CONSTRAINT alerts_acknowledged_by_fkey FOREIGN KEY (acknowledged_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT alerts_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT alerts_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT alerts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT alerts_severity_check CHECK (severity IN ('info', 'warning', 'critical'))
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    user_id uuid,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid,
    old_value jsonb,
    new_value jsonb,
    ip_address text,
    user_agent text,
    created_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT audit_logs_pkey PRIMARY KEY (id),
    CONSTRAINT audit_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL,
    CONSTRAINT audit_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS billing_notices (
    tenant_id uuid NOT NULL,
    kind text NOT NULL,
    episode text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT billing_notices_pkey PRIMARY KEY (tenant_id, kind, episode),
    CONSTRAINT billing_notices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS broadcasts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    body text NOT NULL,
    sent_by uuid,
    sender_name text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT broadcasts_pkey PRIMARY KEY (id),
    CONSTRAINT broadcasts_sent_by_fkey FOREIGN KEY (sent_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT broadcasts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS broadcast_recipients (
    broadcast_id uuid NOT NULL,
    driver_id uuid NOT NULL,
    by_sms boolean DEFAULT false NOT NULL,
    CONSTRAINT broadcast_recipients_pkey PRIMARY KEY (broadcast_id, driver_id),
    CONSTRAINT broadcast_recipients_broadcast_id_fkey FOREIGN KEY (broadcast_id) REFERENCES broadcasts(id) ON DELETE CASCADE,
    CONSTRAINT broadcast_recipients_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS email_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    purpose text NOT NULL,
    email text NOT NULL,
    token_hash text NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT email_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT email_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT email_tokens_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT email_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT email_tokens_purpose_check CHECK (purpose IN ('password_reset', 'email_verify'))
);

CREATE TABLE IF NOT EXISTS emails (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    to_address text NOT NULL,
    subject text NOT NULL,
    body text NOT NULL,
    purpose text NOT NULL,
    sensitive boolean DEFAULT false NOT NULL,
    status text DEFAULT 'queued' NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    provider text,
    next_attempt_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT emails_pkey PRIMARY KEY (id),
    CONSTRAINT emails_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT emails_status_check CHECK (status IN ('queued', 'sending', 'sent', 'failed'))
);

CREATE TABLE IF NOT EXISTS invoice_counters (
    year integer NOT NULL,
    last integer NOT NULL,
    CONSTRAINT invoice_counters_pkey PRIMARY KEY (year)
);

CREATE TABLE IF NOT EXISTS invoices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    number text NOT NULL,
    kind text NOT NULL,
    plan text NOT NULL,
    billing_cycle text NOT NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    lines jsonb NOT NULL,
    usage jsonb,
    subtotal_santim bigint NOT NULL,
    vat_santim bigint NOT NULL,
    total_santim bigint NOT NULL,
    status text DEFAULT 'open' NOT NULL,
    issued_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    customer_name text NOT NULL,
    customer_tin text,
    customer_city text,
    created_by uuid,
    CONSTRAINT invoices_pkey PRIMARY KEY (id),
    CONSTRAINT invoices_number_key UNIQUE (number),
    CONSTRAINT invoices_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT invoices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL,
    CONSTRAINT invoices_billing_cycle_check CHECK (billing_cycle IN ('monthly', 'annual')),
    CONSTRAINT invoices_kind_check CHECK (kind IN ('subscription', 'upgrade', 'renewal', 'usage')),
    CONSTRAINT invoices_status_check CHECK (status IN ('open', 'paid', 'void'))
);

CREATE TABLE IF NOT EXISTS job_runs (
    name text NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    duration_ms integer NOT NULL,
    error text,
    last_ok_at TIMESTAMPTZ,
    runs bigint DEFAULT 1 NOT NULL,
    failures bigint DEFAULT 0 NOT NULL,
    CONSTRAINT job_runs_pkey PRIMARY KEY (name)
);

CREATE TABLE IF NOT EXISTS payments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    invoice_id uuid NOT NULL,
    provider text NOT NULL,
    reference text NOT NULL,
    provider_ref text,
    payer_phone text,
    amount_santim bigint NOT NULL,
    status text DEFAULT 'pending' NOT NULL,
    raw jsonb,
    note text,
    refund_due boolean DEFAULT false NOT NULL,
    refunded_at TIMESTAMPTZ,
    refund_reference text,
    recorded_by uuid,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    completed_at TIMESTAMPTZ,
    checked_at TIMESTAMPTZ,
    CONSTRAINT payments_pkey PRIMARY KEY (id),
    CONSTRAINT payments_reference_key UNIQUE (reference),
    CONSTRAINT payments_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoices(id),
    CONSTRAINT payments_recorded_by_fkey FOREIGN KEY (recorded_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT payments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL,
    CONSTRAINT payments_status_check CHECK (status IN ('pending', 'paid', 'failed', 'expired', 'refunded'))
);

CREATE TABLE IF NOT EXISTS pending_signups (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    token_hash text NOT NULL,
    company text NOT NULL,
    city text NOT NULL,
    full_name text NOT NULL,
    email text NOT NULL,
    phone text NOT NULL,
    password_hash text NOT NULL,
    plan text DEFAULT 'trial' NOT NULL,
    billing_cycle text DEFAULT 'monthly' NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT pending_signups_pkey PRIMARY KEY (id),
    CONSTRAINT pending_signups_token_hash_key UNIQUE (token_hash),
    CONSTRAINT pending_signups_billing_cycle_check CHECK (billing_cycle IN ('monthly', 'annual'))
);

CREATE TABLE IF NOT EXISTS proof_of_deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    driver_id uuid NOT NULL,
    recipient_name text NOT NULL,
    recipient_phone text,
    signature_data text,
    signature_url text,
    photo_urls text[],
    delivery_address text,
    delivery_coordinates geometry(Point,4326),
    delivery_notes text,
    location_verified boolean DEFAULT false,
    location_mismatch_meters numeric(10,2),
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ,
    client_id text,
    status text DEFAULT 'accepted' NOT NULL,
    held_reason text,
    reviewed_at TIMESTAMPTZ,
    reviewed_by uuid,
    location_reason text,
    CONSTRAINT proof_of_deliveries_pkey PRIMARY KEY (id),
    CONSTRAINT proof_of_deliveries_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id),
    CONSTRAINT proof_of_deliveries_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT proof_of_deliveries_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT proof_of_deliveries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT proof_of_deliveries_status_check CHECK (status IN ('accepted', 'held', 'dismissed'))
);

CREATE TABLE IF NOT EXISTS saved_filters (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    query text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT saved_filters_pkey PRIMARY KEY (id),
    CONSTRAINT saved_filters_user_id_name_key UNIQUE (user_id, name),
    CONSTRAINT saved_filters_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT saved_filters_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT saved_filters_name_check CHECK ((length(name) >= 1) AND (length(name) <= 40))
);

CREATE TABLE IF NOT EXISTS sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    refresh_token_hash text NOT NULL,
    device_id text,
    user_agent text,
    ip_address text,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    kind text DEFAULT 'api' NOT NULL,
    remember boolean DEFAULT false NOT NULL,
    last_seen_at TIMESTAMPTZ,
    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT sessions_refresh_token_hash_key UNIQUE (refresh_token_hash),
    CONSTRAINT sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT sessions_kind_check CHECK (kind IN ('api', 'web'))
);

CREATE TABLE IF NOT EXISTS shipment_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    event_type text NOT NULL,
    from_status text,
    to_status text,
    driver_id uuid,
    previous_driver_id uuid,
    actor_id uuid,
    actor_name text DEFAULT '' NOT NULL,
    source text DEFAULT 'dashboard' NOT NULL,
    note text,
    reason text,
    latitude double precision,
    longitude double precision,
    visible_to_driver boolean DEFAULT false NOT NULL,
    public boolean DEFAULT false NOT NULL,
    details jsonb,
    client_id text,
    occurred_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT shipment_events_pkey PRIMARY KEY (id),
    CONSTRAINT shipment_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipment_events_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipment_events_previous_driver_id_fkey FOREIGN KEY (previous_driver_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipment_events_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT shipment_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT shipment_events_event_type_check CHECK (event_type IN ('created', 'edited', 'assigned', 'unassigned', 'status_changed', 'status_updated', 'note', 'message', 'pod_recorded', 'pod_held', 'pod_accepted', 'pod_dismissed', 'customer_confirmed', 'customer_issue', 'conflict')),
    CONSTRAINT shipment_events_source_check CHECK (source IN ('dashboard', 'app', 'sms', 'system', 'customer', 'api'))
);

CREATE TABLE IF NOT EXISTS sms_messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid,
    shipment_id uuid,
    direction text NOT NULL,
    phone text NOT NULL,
    body text NOT NULL,
    purpose text NOT NULL,
    sensitive boolean DEFAULT false NOT NULL,
    status text DEFAULT 'queued' NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    provider text,
    next_attempt_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    provider_message_id text,
    delivery text,
    delivery_at TIMESTAMPTZ,
    segments integer DEFAULT 1 NOT NULL,
    CONSTRAINT sms_messages_pkey PRIMARY KEY (id),
    CONSTRAINT sms_messages_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE SET NULL,
    CONSTRAINT sms_messages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT sms_messages_delivery_check CHECK (delivery IN ('delivered', 'undelivered')),
    CONSTRAINT sms_messages_direction_check CHECK (direction IN ('outbound', 'inbound')),
    CONSTRAINT sms_messages_segments_check CHECK (segments > 0),
    CONSTRAINT sms_messages_status_check CHECK (status IN ('queued', 'sending', 'sent', 'failed', 'received', 'skipped'))
);

CREATE TABLE IF NOT EXISTS sms_opt_outs (
    phone text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT sms_opt_outs_pkey PRIMARY KEY (phone)
);

CREATE TABLE IF NOT EXISTS support_tickets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    number bigint NOT NULL GENERATED ALWAYS AS IDENTITY (START WITH 1001),
    subject text NOT NULL,
    status text DEFAULT 'open' NOT NULL,
    opened_by uuid,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    closed_at TIMESTAMPTZ,
    CONSTRAINT support_tickets_pkey PRIMARY KEY (id),
    CONSTRAINT support_tickets_number_key UNIQUE (number),
    CONSTRAINT support_tickets_opened_by_fkey FOREIGN KEY (opened_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT support_tickets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT support_tickets_status_check CHECK (status IN ('open', 'answered', 'closed')),
    CONSTRAINT support_tickets_subject_check CHECK ((length(subject) >= 1) AND (length(subject) <= 200))
);

CREATE TABLE IF NOT EXISTS support_messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    ticket_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    author_id uuid,
    author_name text NOT NULL,
    from_platform boolean DEFAULT false NOT NULL,
    body text NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT support_messages_pkey PRIMARY KEY (id),
    CONSTRAINT support_messages_author_id_fkey FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT support_messages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT support_messages_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES support_tickets(id) ON DELETE CASCADE,
    CONSTRAINT support_messages_body_check CHECK ((length(body) >= 1) AND (length(body) <= 5000))
);

CREATE TABLE IF NOT EXISTS sync_metadata (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    driver_id uuid NOT NULL,
    last_sync_at TIMESTAMPTZ,
    last_event_id uuid,
    pending_sync_count integer DEFAULT 0,
    storage_used_kb integer DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT now(),
    device_id text,
    app_version text,
    battery_level integer,
    clock_offset_seconds integer,
    tracking_paused boolean DEFAULT false NOT NULL,
    power_saving boolean DEFAULT false NOT NULL,
    CONSTRAINT sync_metadata_pkey PRIMARY KEY (id),
    CONSTRAINT sync_metadata_driver_id_key UNIQUE (driver_id),
    CONSTRAINT sync_metadata_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tracking_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    driver_id uuid,
    coordinates geometry(Point,4326) NOT NULL,
    accuracy_meters numeric(10,2),
    speed_kph numeric(10,2),
    heading numeric(5,2),
    event_type text NOT NULL,
    status text,
    note text,
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ,
    device_id text,
    battery_level integer,
    client_id text,
    CONSTRAINT tracking_events_pkey PRIMARY KEY (id),
    CONSTRAINT tracking_events_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT tracking_events_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT tracking_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT tracking_events_event_type_check CHECK (event_type IN ('gps_ping', 'checkpoint', 'status_change'))
);

CREATE TABLE IF NOT EXISTS verification_codes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    phone text NOT NULL,
    purpose text NOT NULL,
    code_hash text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT verification_codes_pkey PRIMARY KEY (id),
    CONSTRAINT verification_codes_purpose_check CHECK (purpose IN ('pin_reset', 'staff_reset', 'track_lookup', 'signup'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_episode ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id), episode) WHERE (episode IS NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_open_subject ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id)) WHERE (acknowledged_at IS NULL);
CREATE INDEX IF NOT EXISTS idx_alerts_tenant_created ON alerts (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs (action);
CREATE INDEX IF NOT EXISTS idx_audit_created_desc ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_logs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_broadcast_recipients_driver ON broadcast_recipients (driver_id);
CREATE INDEX IF NOT EXISTS idx_broadcasts_tenant ON broadcasts (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_tokens_user ON email_tokens (user_id, purpose, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_emails_queue ON emails (next_attempt_at) WHERE (status = 'queued'::text);
CREATE INDEX IF NOT EXISTS idx_emails_tenant ON emails (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_open ON invoices (tenant_id, due_at) WHERE (status = 'open'::text);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices (tenant_id, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_invoice ON payments (invoice_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_paid ON payments (completed_at) WHERE (status = 'paid'::text);
CREATE INDEX IF NOT EXISTS idx_payments_pending ON payments (created_at) WHERE (status = 'pending'::text);
CREATE INDEX IF NOT EXISTS idx_payments_refund_due ON payments (created_at) WHERE refund_due;
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_accepted ON proof_of_deliveries (shipment_id) WHERE (status = 'accepted'::text);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_client ON proof_of_deliveries (driver_id, client_id) WHERE (client_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_pod_driver ON proof_of_deliveries (driver_id);
CREATE INDEX IF NOT EXISTS idx_pod_shipment ON proof_of_deliveries (shipment_id);
CREATE INDEX IF NOT EXISTS idx_pod_tenant ON proof_of_deliveries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_saved_filters_user ON saved_filters (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shipment_events_client ON shipment_events (shipment_id, client_id) WHERE (client_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_shipment_events_driver_feed ON shipment_events (driver_id, created_at) WHERE visible_to_driver;
CREATE INDEX IF NOT EXISTS idx_shipment_events_shipment ON shipment_events (shipment_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_shipment_events_tenant ON shipment_events (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shipment_events_unassigned ON shipment_events (previous_driver_id, created_at) WHERE (previous_driver_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_shipments_created ON shipments (created_at);
CREATE INDEX IF NOT EXISTS idx_shipments_customer_phone ON shipments (customer_phone);
CREATE INDEX IF NOT EXISTS idx_shipments_driver ON shipments (driver_id);
CREATE INDEX IF NOT EXISTS idx_shipments_pickup_due ON shipments (planned_pickup_at) WHERE ((status = ANY (ARRAY['pending'::text, 'assigned'::text])) AND (planned_pickup_at IS NOT NULL));
CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments (status);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_created ON shipments (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_delivered ON shipments (tenant_id, actual_delivery) WHERE (status = 'delivered'::text);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_status ON shipments (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_shipments_to_anonymise ON shipments (updated_at) WHERE ((anonymised_at IS NULL) AND (status = ANY (ARRAY['delivered'::text, 'cancelled'::text])));
CREATE INDEX IF NOT EXISTS idx_sms_otp_created ON sms_messages (created_at) WHERE (purpose = 'otp'::text);
CREATE INDEX IF NOT EXISTS idx_sms_phone_created ON sms_messages (phone, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_provider_message ON sms_messages (provider, direction, provider_message_id) WHERE (provider_message_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_sms_queue ON sms_messages (next_attempt_at) WHERE (status = 'queued'::text);
CREATE INDEX IF NOT EXISTS idx_sms_tenant ON sms_messages (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_messages_ticket ON support_messages (ticket_id, created_at);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_tenant ON support_tickets (tenant_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_tracking_coords ON tracking_events USING gist (coordinates);
CREATE INDEX IF NOT EXISTS idx_tracking_driver ON tracking_events (driver_id);
CREATE INDEX IF NOT EXISTS idx_tracking_driver_recorded ON tracking_events (tenant_id, driver_id, recorded_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tracking_events_device_client ON tracking_events (device_id, client_id) WHERE ((client_id IS NOT NULL) AND (device_id IS NOT NULL));
CREATE INDEX IF NOT EXISTS idx_tracking_recorded ON tracking_events (recorded_at);
CREATE INDEX IF NOT EXISTS idx_tracking_shipment_recorded ON tracking_events (shipment_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users (email) WHERE (email IS NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON users (phone) WHERE (phone IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_verification_codes_lookup ON verification_codes (phone, purpose, created_at DESC);

CREATE OR REPLACE TRIGGER audit_logs_append_only BEFORE DELETE OR UPDATE ON audit_logs FOR EACH ROW EXECUTE FUNCTION append_only();
CREATE OR REPLACE TRIGGER audit_logs_no_truncate BEFORE TRUNCATE ON audit_logs FOR EACH STATEMENT EXECUTE FUNCTION append_only();
CREATE OR REPLACE TRIGGER shipment_events_append_only BEFORE DELETE OR UPDATE ON shipment_events FOR EACH ROW EXECUTE FUNCTION append_only();
CREATE OR REPLACE TRIGGER shipment_events_no_truncate BEFORE TRUNCATE ON shipment_events FOR EACH STATEMENT EXECUTE FUNCTION append_only();
CREATE OR REPLACE TRIGGER update_shipments_updated_at BEFORE UPDATE ON shipments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE OR REPLACE TRIGGER update_sync_metadata_updated_at BEFORE UPDATE ON sync_metadata FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE OR REPLACE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE OR REPLACE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE OR REPLACE TRIGGER update_vehicles_updated_at BEFORE UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TABLE IF EXISTS verification_codes CASCADE;
DROP TABLE IF EXISTS tracking_events CASCADE;
DROP TABLE IF EXISTS sync_metadata CASCADE;
DROP TABLE IF EXISTS support_messages CASCADE;
DROP TABLE IF EXISTS support_tickets CASCADE;
DROP TABLE IF EXISTS sms_opt_outs CASCADE;
DROP TABLE IF EXISTS sms_messages CASCADE;
DROP TABLE IF EXISTS shipment_events CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS saved_filters CASCADE;
DROP TABLE IF EXISTS proof_of_deliveries CASCADE;
DROP TABLE IF EXISTS pending_signups CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS job_runs CASCADE;
DROP TABLE IF EXISTS invoices CASCADE;
DROP TABLE IF EXISTS invoice_counters CASCADE;
DROP TABLE IF EXISTS emails CASCADE;
DROP TABLE IF EXISTS email_tokens CASCADE;
DROP TABLE IF EXISTS broadcast_recipients CASCADE;
DROP TABLE IF EXISTS broadcasts CASCADE;
DROP TABLE IF EXISTS billing_notices CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS alerts CASCADE;
DROP TABLE IF EXISTS shipments CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS vehicles CASCADE;
DROP TABLE IF EXISTS tenants CASCADE;
DROP FUNCTION IF EXISTS append_only();
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP EXTENSION IF EXISTS postgis CASCADE;
