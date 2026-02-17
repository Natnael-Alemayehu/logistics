package validation

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

type TestStruct struct {
	PIN string `validate:"required,pin"`
}

func TestValidatePIN(t *testing.T) {
	tests := []struct {
		name    string
		pin     string
		wantErr bool
	}{
		{"valid 4 digit", "1234", false},
		{"valid 5 digit", "12345", false},
		{"valid 6 digit", "123456", false},
		{"too short 3 digit", "123", true},
		{"too long 7 digit", "1234567", true},
		{"empty", "", true},
		{"contains letter", "12a4", true},
		{"contains special char", "12@4", true},
	}

	Init()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestStruct{PIN: tt.pin}
			err := Get().Struct(testObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePIN(%s) error = %v, wantErr %v", tt.pin, err, tt.wantErr)
			}
		})
	}
}

func TestFormatErrors(t *testing.T) {
	Init()

	type TestPIN struct {
		PIN string `validate:"required,pin"`
	}

	err := Get().Struct(TestPIN{PIN: "123"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	errors := FormatErrors(err.(validator.ValidationErrors))
	if len(errors) == 0 {
		t.Fatal("expected formatted errors")
	}

	if errors[0].Field != "PIN" {
		t.Errorf("expected field PIN, got %s", errors[0].Field)
	}

	if errors[0].Message != "PIN must be 4-6 digits" {
		t.Errorf("expected 'PIN must be 4-6 digits', got %s", errors[0].Message)
	}
}
