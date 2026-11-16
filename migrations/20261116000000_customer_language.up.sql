-- +goose Up
-- The language a company's customers are written to in: the texts they are
-- sent, and the tracking and confirmation pages until they choose another.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS customer_language text DEFAULT 'en' NOT NULL;
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_customer_language_check;
ALTER TABLE tenants ADD CONSTRAINT tenants_customer_language_check CHECK (customer_language IN ('en', 'am'));

-- +goose Down
ALTER TABLE tenants DROP COLUMN IF EXISTS customer_language;
