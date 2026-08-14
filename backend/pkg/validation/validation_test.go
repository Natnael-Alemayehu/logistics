package validation

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestPIN struct {
	PIN string `validate:"required,pin"`
}

type TestPhone struct {
	Phone string `validate:"required,ethiopian_phone"`
}

type TestEmail struct {
	Email string `validate:"required,email"`
}

type TestMinMax struct {
	Name string `validate:"required,min=2,max=10"`
}

type TestOneOf struct {
	Role string `validate:"required,oneof=admin dispatcher driver"`
}

type TestUUID struct {
	ID string `validate:"required,uuid"`
}

type TestRequired struct {
	Field string `validate:"required"`
}

type TestMultiple struct {
	Phone string `validate:"required,ethiopian_phone"`
	PIN   string `validate:"required,pin"`
	Email string `validate:"required,email"`
}

func TestMain(m *testing.M) {
	Init()
	m.Run()
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
		{"contains space", "12 4", true},
		{"all zeros", "0000", false},
		{"all same digits", "111111", false},
		{"mixed valid length 5", "54321", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestPIN{PIN: tt.pin}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEthiopianPhone(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{"valid 09 format", "0912345678", false},
		{"valid 07 format", "0712345678", false},
		{"valid +251 9 format", "+251912345678", false},
		{"valid +251 7 format", "+251712345678", false},
		{"valid 251 9 format (no plus)", "251912345678", false},
		{"valid 251 7 format (no plus)", "251712345678", false},
		{"invalid 08 format", "0812345678", true},
		{"invalid 06 format", "0612345678", true},
		{"too short local", "091234567", true},
		{"too long local", "09123456789", true},
		{"too short international", "+25191234567", true},
		{"too long international", "+2519123456789", true},
		{"empty", "", true},
		{"invalid format", "1234567890", true},
		{"international without plus", "2519123456789", true},
		{"international wrong prefix", "+252912345678", true},
		{"international +251 with 08", "+251812345678", true},
		{"international +251 with 06", "+251612345678", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestPhone{Phone: tt.phone}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeEthiopianPhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"+2519 format", "+251912345678", "0912345678"},
		{"+2517 format", "+251712345678", "0712345678"},
		{"2519 format (no plus)", "251912345678", "0912345678"},
		{"2517 format (no plus)", "251712345678", "0712345678"},
		{"09 format already local", "0912345678", "0912345678"},
		{"07 format already local", "0712345678", "0712345678"},
		{"+251 with spaces", "+251 9 12345678", "0912345678"},
		{"+251 with dashes", "+251-912-345-678", "0912345678"},
		{"invalid format unchanged", "invalid", "invalid"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEthiopianPhone(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with subdomain", "test@mail.example.com", false},
		{"valid with plus", "test+tag@example.com", false},
		{"valid with dash", "test-name@example.com", false},
		{"invalid no @", "testexample.com", true},
		{"invalid no domain", "test@", true},
		{"invalid no local", "@example.com", true},
		{"invalid double @", "test@@example.com", true},
		{"empty", "", true},
		{"spaces", "test @example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestEmail{Email: tt.email}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMinMax(t *testing.T) {
	tests := []struct {
		name    string
		nameVal string
		wantErr bool
	}{
		{"valid min", "ab", false},
		{"valid mid", "abcde", false},
		{"valid max", "abcdefghij", false},
		{"too short", "a", true},
		{"too long", "abcdefghijk", true},
		{"empty", "", true},
		{"exact min", "ab", false},
		{"exact max", "abcdefghij", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestMinMax{Name: tt.nameVal}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateOneOf(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{"valid admin", "admin", false},
		{"valid dispatcher", "dispatcher", false},
		{"valid driver", "driver", false},
		{"invalid role", "manager", true},
		{"invalid partial", "admin1", true},
		{"empty", "", true},
		{"case sensitive", "Admin", true},
		{"spaces", "admin ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestOneOf{Role: tt.role}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid uuid v4", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid uuid v1", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", false},
		{"invalid format", "not-a-uuid", true},
		{"invalid too short", "550e8400-e29b-41d4-a716", true},
		{"invalid missing dashes", "550e8400e29b41d4a716446655440000", true},
		{"empty", "", true},
		{"invalid chars", "550e8400-e29b-41d4-a716-44665544ZZZZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestUUID{ID: tt.id}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{"has value", "something", false},
		{"empty", "", true},
		{"spaces only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestRequired{Field: tt.field}
			err := Get().Struct(testObj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMultiple(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		pin     string
		email   string
		wantErr bool
		errType string
	}{
		{"all valid", "0912345678", "1234", "test@example.com", false, ""},
		{"invalid phone", "0812345678", "1234", "test@example.com", true, "Phone"},
		{"invalid pin", "0912345678", "12a4", "test@example.com", true, "PIN"},
		{"invalid email", "0912345678", "1234", "invalid-email", true, "Email"},
		{"multiple invalid", "0812345678", "12a4", "invalid-email", true, "multiple"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testObj := TestMultiple{
				Phone: tt.phone,
				PIN:   tt.pin,
				Email: tt.email,
			}
			err := Get().Struct(testObj)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFormatErrors(t *testing.T) {
	t.Run("format PIN error", func(t *testing.T) {
		err := Get().Struct(TestPIN{PIN: "123"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "PIN", errors[0].Field)
		assert.Equal(t, "PIN must be 4-6 digits", errors[0].Message)
	})

	t.Run("format phone error", func(t *testing.T) {
		err := Get().Struct(TestPhone{Phone: "invalid"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Phone", errors[0].Field)
		assert.Contains(t, errors[0].Message, "valid Ethiopian phone number")
	})

	t.Run("format required error", func(t *testing.T) {
		err := Get().Struct(TestRequired{Field: ""})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Field", errors[0].Field)
		assert.Equal(t, "Field is required", errors[0].Message)
	})

	t.Run("format email error", func(t *testing.T) {
		err := Get().Struct(TestEmail{Email: "invalid"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Email", errors[0].Field)
		assert.Contains(t, errors[0].Message, "valid email address")
	})

	t.Run("format min error", func(t *testing.T) {
		err := Get().Struct(TestMinMax{Name: "a"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Name", errors[0].Field)
		assert.Contains(t, errors[0].Message, "at least 2")
	})

	t.Run("format max error", func(t *testing.T) {
		err := Get().Struct(TestMinMax{Name: "abcdefghijk"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Name", errors[0].Field)
		assert.Contains(t, errors[0].Message, "at most 10")
	})

	t.Run("format oneof error", func(t *testing.T) {
		err := Get().Struct(TestOneOf{Role: "invalid"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "Role", errors[0].Field)
		assert.Contains(t, errors[0].Message, "must be one of")
	})

	t.Run("format uuid error", func(t *testing.T) {
		err := Get().Struct(TestUUID{ID: "invalid"})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		require.Len(t, errors, 1)
		assert.Equal(t, "ID", errors[0].Field)
		assert.Contains(t, errors[0].Message, "valid UUID")
	})

	t.Run("format multiple errors", func(t *testing.T) {
		err := Get().Struct(TestMultiple{
			Phone: "invalid",
			PIN:   "123",
			Email: "invalid",
		})
		require.Error(t, err)

		validationErrors, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		errors := FormatErrors(validationErrors)
		assert.GreaterOrEqual(t, len(errors), 2)
	})

	t.Run("no errors", func(t *testing.T) {
		err := Get().Struct(TestPIN{PIN: "1234"})
		require.NoError(t, err)

		errors := FormatErrors(nil)
		assert.Nil(t, errors)
	})
}

func TestGet(t *testing.T) {
	v := Get()
	assert.NotNil(t, v)
}

func TestInit(t *testing.T) {
	Init()
	v := Get()
	assert.NotNil(t, v)

	v2 := Get()
	assert.Equal(t, v, v2)
}

func BenchmarkValidatePIN(b *testing.B) {
	Init()
	testObj := TestPIN{PIN: "1234"}
	for i := 0; i < b.N; i++ {
		_ = Get().Struct(testObj)
	}
}

func BenchmarkValidatePhone(b *testing.B) {
	Init()
	testObj := TestPhone{Phone: "0912345678"}
	for i := 0; i < b.N; i++ {
		_ = Get().Struct(testObj)
	}
}

func BenchmarkFormatErrors(b *testing.B) {
	Init()
	for i := 0; i < b.N; i++ {
		err := Get().Struct(TestPIN{PIN: "123"})
		if err != nil {
			validationErrors, _ := err.(validator.ValidationErrors)
			_ = FormatErrors(validationErrors)
		}
	}
}
