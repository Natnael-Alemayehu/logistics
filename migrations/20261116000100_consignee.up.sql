-- +goose Up
-- The consignee: who takes delivery, when that isn't the customer who booked
-- the shipment (audit W4). Both columns are set or neither is; without them
-- the customer receives, as every shipment did before.
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS consignee_name text;
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS consignee_phone text;
ALTER TABLE shipments DROP CONSTRAINT IF EXISTS shipments_consignee_check;
ALTER TABLE shipments ADD CONSTRAINT shipments_consignee_check CHECK ((consignee_name IS NULL) = (consignee_phone IS NULL));
-- For finding one's shipments by phone, as idx_shipments_customer_phone is.
CREATE INDEX IF NOT EXISTS idx_shipments_consignee_phone ON shipments (consignee_phone) WHERE consignee_phone IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_shipments_consignee_phone;
ALTER TABLE shipments DROP CONSTRAINT IF EXISTS shipments_consignee_check;
ALTER TABLE shipments DROP COLUMN IF EXISTS consignee_phone;
ALTER TABLE shipments DROP COLUMN IF EXISTS consignee_name;
