package service

import (
	"errors"
	"net/http"
)

// Error is a failure the caller can act on: bad input, a missing record, a
// rule that forbids the action. Its message is written for the person using
// the product and is safe to show them. Anything that is not an *Error is an
// internal fault, whose details belong in the log rather than the response.
type Error struct {
	Kind    Kind
	Message string
	// Field names the input field at fault, when there is one, so a form can
	// show the message next to it.
	Field string
	// Key is the message's key in internal/text, set on the errors a
	// customer can meet, so that their pages can say it in Amharic.
	Key string
}

func (e *Error) Error() string { return e.Message }

// Kind classifies an Error for transport mapping.
type Kind int

const (
	KindInvalid Kind = iota
	KindNotFound
	KindConflict
	KindForbidden
	KindUnauthorized
	KindLimit
)

// HTTPStatus maps a Kind to a status code.
func (k Kind) HTTPStatus() int {
	switch k {
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindForbidden:
		return http.StatusForbidden
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindLimit:
		return http.StatusPaymentRequired
	default:
		return http.StatusBadRequest
	}
}

// Code is the stable machine-readable form of a Kind for API clients.
func (k Kind) Code() string {
	switch k {
	case KindNotFound:
		return "NOT_FOUND"
	case KindConflict:
		return "CONFLICT"
	case KindForbidden:
		return "FORBIDDEN"
	case KindUnauthorized:
		return "UNAUTHORIZED"
	case KindLimit:
		return "PLAN_LIMIT"
	default:
		return "VALIDATION_ERROR"
	}
}

func invalid(field, msg string) error { return &Error{Kind: KindInvalid, Field: field, Message: msg} }
func notFound(msg string) error       { return &Error{Kind: KindNotFound, Message: msg} }
func conflict(msg string) error       { return &Error{Kind: KindConflict, Message: msg} }
func forbidden(msg string) error      { return &Error{Kind: KindForbidden, Message: msg} }
func limitReached(msg string) error   { return &Error{Kind: KindLimit, Message: msg} }

// AsError extracts a domain error, if err is one.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// IsNotFound reports whether err is a not-found domain error.
func IsNotFound(err error) bool {
	e, ok := AsError(err)
	return ok && e.Kind == KindNotFound
}

var (
	ErrAccountLocked      = &Error{Kind: KindForbidden, Message: "Too many failed attempts. The account is locked for 15 minutes."}
	ErrInvalidCredentials = &Error{Kind: KindUnauthorized, Message: "Incorrect login details."}
	ErrAccountInactive    = &Error{Kind: KindForbidden, Message: "This account has been deactivated. Contact your administrator."}
	ErrSessionRevoked     = &Error{Kind: KindUnauthorized, Message: "Your session has ended. Please sign in again."}
	ErrWrongApp           = &Error{Kind: KindForbidden, Message: "Drivers sign in through the driver app."}
	ErrNotDriver          = &Error{Kind: KindForbidden, Message: "Only drivers can use the driver app."}
	ErrSamePassword       = &Error{Kind: KindInvalid, Field: "new_password", Message: "Choose a password different from the current one."}
	ErrUnauthorized       = &Error{Kind: KindForbidden, Message: "You do not have permission to do that."}
	errAnonymised         = &Error{Kind: KindConflict, Message: "This person's details were removed, so the account can't be used again. Add them as a new person instead."}
	errShipmentAnonymised = &Error{Kind: KindConflict, Message: "This shipment's customer and delivery details were removed after the keeping period, so it can't be changed. Book a new shipment instead."}
	ErrNotAssigned        = &Error{Kind: KindForbidden, Message: "This shipment is not assigned to you."}
	ErrStaleUpdate        = &Error{Kind: KindConflict, Message: "The shipment was changed while you were offline, so this update was not applied."}
)

// suspended is the error for a suspended company's users: they can't sign in
// to ask on the Help page, so it says where to write.
func (c *core) suspended() error {
	return &Error{Kind: KindForbidden, Message: "This company account is suspended. Write to " + c.cfg.SupportEmail + " to have it restored."}
}
