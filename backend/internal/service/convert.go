package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// errEmptyUUID is returned by parseUUID for an empty identifier, which is
// distinct from a malformed one: callers that treat a missing ID as optional
// can check for it explicitly.
var errEmptyUUID = errors.New("empty uuid")

// parseUUID converts a UUID string to its pgtype representation, reporting
// malformed input instead of silently yielding a NULL value.
//
// Prefer this wherever the identifier came from a client. Writing an invalid
// pgtype.UUID to a NOT NULL column fails at the database, which reads as an
// opaque insert error rather than the bad input it actually is.
func parseUUID(s string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, errEmptyUUID
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// toUUID converts a UUID string, yielding an invalid (SQL NULL) value for input
// it cannot parse. Callers that need to surface bad input should use parseUUID.
func toUUID(s string) pgtype.UUID {
	id, err := parseUUID(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return id
}

func toText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// toNumeric converts a float to pgtype.Numeric, preserving zero.
//
// pgtype.Numeric.Scan accepts only strings and nil — handed a float64 it always
// returns "cannot scan float64" — so the value must be formatted first. Getting
// this wrong wrote NULL for every reading that passed through it.
//
// Zero is preserved because for telemetry it is a real measurement: a parked
// truck genuinely reports a speed of 0. Use toNumericOmitZero where zero instead
// means "not supplied".
func toNumeric(f float64) pgtype.Numeric {
	// Postgres NUMERIC accepts NaN, so Scan would happily store it. A NaN reading
	// from a bad GPS fix would then propagate through every AVG() over the
	// column, so drop it to NULL instead. Infinity is rejected by Scan itself.
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{}
	}

	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// toNumericOmitZero converts a float to pgtype.Numeric, mapping zero to NULL.
// Use it for optional quantities where zero and "unspecified" are the same
// thing, such as cargo weight and value.
func toNumericOmitZero(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{}
	}
	return toNumeric(f)
}

func toTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func toInt32Ptr(i int) *int32 {
	if i == 0 {
		return nil
	}
	v := int32(i)
	return &v
}

func toBoolPtr(b bool) *bool {
	return &b
}
