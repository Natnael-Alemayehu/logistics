package validation

import (
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

func validateEthiopianPhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if len(phone) == 10 && (phone[:2] == "09" || phone[:2] == "07") {
		return true
	}
	if len(phone) == 13 && phone[:4] == "+251" && (phone[4:5] == "9" || phone[4:5] == "7") {
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
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Error string `json:"error"`
}

func FormatErrors(err error) []ValidationError {
	var errors []ValidationError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errors = append(errors, ValidationError{
				Field: e.Field(),
				Tag:   e.Tag(),
				Error: e.Error(),
			})
		}
	}
	return errors
}
