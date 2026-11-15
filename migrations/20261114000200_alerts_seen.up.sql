-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Each person's alert badge is their own: it counts open alerts raised since
-- they last opened the alert list. Dismissing an alert still clears it for
-- everyone, as it is then dealt with.
-- ---------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS alerts_seen_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS alerts_seen_at;
-- +goose StatementEnd
