-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- When a company admin turned off renewal. The plan then runs to period_end
-- and isn't renewed; after that the account is read-only until a plan is
-- chosen again. NULL while the subscription renews.
-- ---------------------------------------------------------------------------
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS renewal_cancelled_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tenants DROP COLUMN IF EXISTS renewal_cancelled_at;
-- +goose StatementEnd
