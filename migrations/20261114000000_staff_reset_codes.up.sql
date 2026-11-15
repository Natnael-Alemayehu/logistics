-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Staff who sign in with a phone, or whose email address isn't confirmed,
-- reset their password with a code texted to that phone.
-- ---------------------------------------------------------------------------
ALTER TABLE verification_codes DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;
ALTER TABLE verification_codes ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('pin_reset', 'staff_reset', 'track_lookup', 'signup'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM verification_codes WHERE purpose = 'staff_reset';
ALTER TABLE verification_codes DROP CONSTRAINT IF EXISTS verification_codes_purpose_check;
ALTER TABLE verification_codes ADD CONSTRAINT verification_codes_purpose_check
    CHECK (purpose IN ('pin_reset', 'track_lookup', 'signup'));
-- +goose StatementEnd
