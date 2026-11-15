-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- The setting that lets a transaction change the append-only tables was
-- renamed along with the product. A new database gets this function from the
-- migration that first creates it. A database made before the rename still
-- has the function reading the old setting, and would refuse every purge:
-- deleting a company, and removing personal data after the retention period.
-- Recreating the function brings it in line.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION append_only() RETURNS trigger AS $$
BEGIN
    IF current_setting('kiflet.purge', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Nothing to undo: this is the function the earlier migration creates.
SELECT 1;
-- +goose StatementEnd
