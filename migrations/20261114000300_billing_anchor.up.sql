-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- The day of the month a company's periods end on. A month from 31 January
-- ends on 28 February; without the day kept, the next ended on 28 March and
-- every one after on the 28th. Set when a period starts afresh; NULL for
-- companies not yet renewed since, whose day is worked out on renewal.
-- ---------------------------------------------------------------------------
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS billing_anchor_day INTEGER
    CHECK (billing_anchor_day BETWEEN 1 AND 31);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tenants DROP COLUMN IF EXISTS billing_anchor_day;
-- +goose StatementEnd
