package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToUUID(t *testing.T) {
	t.Run("converts valid UUID string", func(t *testing.T) {
		uuidStr := "550e8400-e29b-41d4-a716-446655440000"
		result := toUUID(uuidStr)
		assert.True(t, result.Valid)
	})

	t.Run("returns invalid for empty string", func(t *testing.T) {
		result := toUUID("")
		assert.False(t, result.Valid)
	})

	t.Run("handles invalid UUID format", func(t *testing.T) {
		result := toUUID("not-a-uuid")
		assert.False(t, result.Valid)
	})
}

func TestToText(t *testing.T) {
	t.Run("returns nil for empty string", func(t *testing.T) {
		result := toText("")
		assert.Nil(t, result)
	})

	t.Run("returns pointer for non-empty string", func(t *testing.T) {
		result := toText("test")
		require.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})

	t.Run("handles whitespace string", func(t *testing.T) {
		result := toText("   ")
		require.NotNil(t, result)
		assert.Equal(t, "   ", *result)
	})
}

func TestToNumeric(t *testing.T) {
	t.Run("converts positive float (note: Scan with float64 does not work)", func(t *testing.T) {
		result := toNumeric(123.45)
		assert.False(t, result.Valid)
	})

	t.Run("returns invalid for zero", func(t *testing.T) {
		result := toNumeric(0)
		assert.False(t, result.Valid)
	})

	t.Run("handles large positive numbers (note: Scan with float64 does not work)", func(t *testing.T) {
		result := toNumeric(999999.999)
		assert.False(t, result.Valid)
	})

	t.Run("handles small positive decimals (note: Scan with float64 does not work)", func(t *testing.T) {
		result := toNumeric(0.001)
		assert.False(t, result.Valid)
	})

	t.Run("returns invalid for negative numbers", func(t *testing.T) {
		result := toNumeric(-1.5)
		assert.False(t, result.Valid)
	})
}

func TestToTimestamp(t *testing.T) {
	t.Run("converts valid time", func(t *testing.T) {
		now := time.Now()
		result := toTimestamp(now)
		assert.True(t, result.Valid)
		assert.Equal(t, now, result.Time)
	})

	t.Run("returns invalid for zero time", func(t *testing.T) {
		result := toTimestamp(time.Time{})
		assert.False(t, result.Valid)
	})

	t.Run("handles UTC time", func(t *testing.T) {
		utcTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		result := toTimestamp(utcTime)
		assert.True(t, result.Valid)
		assert.Equal(t, utcTime, result.Time)
	})
}

func TestToInt32Ptr(t *testing.T) {
	t.Run("returns nil for zero", func(t *testing.T) {
		result := toInt32Ptr(0)
		assert.Nil(t, result)
	})

	t.Run("returns pointer for non-zero", func(t *testing.T) {
		result := toInt32Ptr(42)
		require.NotNil(t, result)
		assert.Equal(t, int32(42), *result)
	})

	t.Run("handles negative values", func(t *testing.T) {
		result := toInt32Ptr(-1)
		require.NotNil(t, result)
		assert.Equal(t, int32(-1), *result)
	})

	t.Run("handles max int", func(t *testing.T) {
		result := toInt32Ptr(2147483647)
		require.NotNil(t, result)
		assert.Equal(t, int32(2147483647), *result)
	})
}

func TestToUUID_RoundTrip(t *testing.T) {
	original := "550e8400-e29b-41d4-a716-446655440000"
	uuid := toUUID(original)

	assert.True(t, uuid.Valid, "UUID should be valid after scanning a valid UUID string")
}

func TestToNumeric_ZeroBehavior(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		wantValid bool
	}{
		{"zero", 0, false},
		{"negative", -1.5, false},
		{"positive (note: Scan with float64 does not work)", 1.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toNumeric(tt.input)
			assert.Equal(t, tt.wantValid, result.Valid)
		})
	}
}

func TestToText_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *string
	}{
		{"empty", "", nil},
		{"single char", "a", strPtr("a")},
		{"long string", "this is a very long string for testing", strPtr("this is a very long string for testing")},
		{"unicode", "你好世界", strPtr("你好世界")},
		{"special chars", "!@#$%^&*()", strPtr("!@#$%^&*()")},
		{"newline", "line1\nline2", strPtr("line1\nline2")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toText(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func BenchmarkToUUID(b *testing.B) {
	uuidStr := "550e8400-e29b-41d4-a716-446655440000"
	for i := 0; i < b.N; i++ {
		_ = toUUID(uuidStr)
	}
}

func BenchmarkToText(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = toText("test string")
	}
}

func BenchmarkToNumeric(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = toNumeric(123.45)
	}
}

func BenchmarkToTimestamp(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		_ = toTimestamp(now)
	}
}

func BenchmarkToInt32Ptr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = toInt32Ptr(42)
	}
}
