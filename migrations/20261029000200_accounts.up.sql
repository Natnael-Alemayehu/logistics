-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Staff confirm their email address, since password resets, invoices and
-- receipts are sent there. Accounts that already existed count as confirmed.
-- ---------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
UPDATE users SET email_verified_at = created_at WHERE email IS NOT NULL AND email_verified_at IS NULL;

-- ---------------------------------------------------------------------------
-- Links sent by email: password resets and address confirmations. Only a
-- hash of the token is kept, and each works once. The address is recorded so
-- a confirmation link stops counting if the address changes before it is
-- followed.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS email_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('password_reset', 'email_verify')),
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_tokens_user ON email_tokens(user_id, purpose, created_at DESC);

-- ---------------------------------------------------------------------------
-- A sign-up waiting for its phone number to be confirmed by SMS code. The
-- company is only created once the code comes back. The plan chosen at
-- sign-up decides between the free trial and paying at once.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pending_signups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    company TEXT NOT NULL,
    city TEXT NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    plan TEXT NOT NULL DEFAULT 'trial',
    billing_cycle TEXT NOT NULL DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'annual')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE verification_codes DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;
ALTER TABLE verification_codes ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('pin_reset', 'track_lookup', 'signup'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM verification_codes WHERE purpose = 'signup';
ALTER TABLE verification_codes DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;
ALTER TABLE verification_codes ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('pin_reset', 'track_lookup'));
DROP TABLE IF EXISTS pending_signups;
DROP TABLE IF EXISTS email_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
-- +goose StatementEnd
