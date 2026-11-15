// Package phone normalises Ethiopian mobile numbers.
//
// Numbers are stored in the local ten-digit form (09xxxxxxxx or 07xxxxxxxx),
// which is how people write and say them, and converted to E.164 only when an
// SMS provider needs it. Every place a number enters the system goes through
// Normalize, so the same phone always compares equal.
package phone

import (
	"errors"
	"strings"
)

// ErrInvalid is returned for anything that is not an Ethiopian mobile number.
var ErrInvalid = errors.New("enter an Ethiopian mobile number, for example 0911234567")

// Normalize returns the local ten-digit form of an Ethiopian mobile number. It
// accepts the forms people actually type: 0911234567, 911234567,
// +251911234567, 251911234567, with spaces, dashes, dots or brackets.
func Normalize(s string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", ErrInvalid
		}
	}
	d := b.String()

	switch {
	case len(d) == 12 && strings.HasPrefix(d, "251"):
		d = "0" + d[3:]
	case len(d) == 9:
		d = "0" + d
	}
	if len(d) != 10 || d[0] != '0' || (d[1] != '9' && d[1] != '7') {
		return "", ErrInvalid
	}
	return d, nil
}

// Mask hides the middle of a number for display to someone who should
// recognise it but not learn it, such as a customer looking up shipments.
func Mask(local string) string {
	if len(local) != 10 {
		return local
	}
	return local[:4] + "••••" + local[8:]
}
