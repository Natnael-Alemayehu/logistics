-- +goose Up
-- How many pieces a load is, and what they are ("152 bales"): the driver
-- counts them at pickup and the receiver at delivery.
ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS cargo_pieces integer CHECK (cargo_pieces > 0),
    ADD COLUMN IF NOT EXISTS cargo_unit text CHECK (char_length(cargo_unit) <= 30);

-- +goose Down
ALTER TABLE shipments DROP COLUMN IF EXISTS cargo_pieces, DROP COLUMN IF EXISTS cargo_unit;
