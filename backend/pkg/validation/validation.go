// Package validation validates
package validation

import (
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func Init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterValidation("ethiopian_phone", validateEthiopianPhone)
	validate.RegisterValidation("pin", validatePIN)
}

func Get() *validator.Validate {
	return validate
}

var nonDigitRegex = regexp.MustCompile(`\D`)

func NormalizeEthiopianPhone(phone string) string {
	digits := nonDigitRegex.ReplaceAllString(phone, "")

	if len(digits) == 12 && digits[:3] == "251" {
		return "0" + digits[3:]
	}
	if len(digits) == 10 && (digits[:2] == "09" || digits[:2] == "07") {
		return digits
	}
	return phone
}

func validateEthiopianPhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if len(phone) == 10 && (phone[:2] == "09" || phone[:2] == "07") {
		return true
	}
	if len(phone) == 13 && phone[:4] == "+251" && (phone[4:5] == "9" || phone[4:5] == "7") {
		return true
	}
	if len(phone) == 12 && phone[:3] == "251" && (phone[3:4] == "9" || phone[3:4] == "7") {
		return true
	}
	return false
}

func validatePIN(fl validator.FieldLevel) bool {
	pin := fl.Field().String()
	if len(pin) < 4 || len(pin) > 6 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func FormatErrors(err error) []ValidationError {
	var errors []ValidationError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errors = append(errors, ValidationError{
				Field:   e.Field(),
				Message: getErrorMessage(e),
			})
		}
	}
	return errors
}

func getErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "ethiopian_phone":
		return fmt.Sprintf("%s must be a valid Ethiopian phone number (09XXXXXXXX or 07XXXXXXXX)", e.Field())
	case "pin":
		return fmt.Sprintf("%s must be 4-6 digits", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", e.Field())
	default:
		return fmt.Sprintf("%s is invalid", e.Field())
	}
}
