package service

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func toUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	u.Scan(s)
	return u
}

func toText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if f > 0 {
		n.Scan(f)
	}
	return n
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
