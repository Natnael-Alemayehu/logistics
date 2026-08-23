package service

import (
	"regexp"
	"strings"
	"testing"
)

// The tracking number is the only secret protecting the public, unauthenticated
// tracking endpoint, so these tests guard its unpredictability rather than just
// its shape. The previous generator derived the suffix from the clock and gave
// 10,000 values per day, which made every shipment in the system enumerable.
func TestGenerateTrackingNumber(t *testing.T) {
	format := regexp.MustCompile(`^ET-\d{8}-[` + trackingCodeAlphabet + `]{10}$`)

	t.Run("matches the expected format", func(t *testing.T) {
		number, err := generateTrackingNumber()
		if err != nil {
			t.Fatalf("generateTrackingNumber() error = %v", err)
		}
		if !format.MatchString(number) {
			t.Errorf("tracking number %q does not match %s", number, format)
		}
	})

	t.Run("avoids characters that are misread aloud", func(t *testing.T) {
		number, err := generateTrackingNumber()
		if err != nil {
			t.Fatalf("generateTrackingNumber() error = %v", err)
		}

		code := number[strings.LastIndex(number, "-")+1:]
		for _, ambiguous := range []string{"I", "L", "O", "U"} {
			if strings.Contains(code, ambiguous) {
				t.Errorf("code %q contains ambiguous character %q; customers read these over the phone", code, ambiguous)
			}
		}
	})

	t.Run("does not repeat across rapid successive calls", func(t *testing.T) {
		// The old generator collided for any two shipments created within the
		// same 100 microseconds, which the UNIQUE constraint turned into a
		// failed creation.
		const issueCount = 2000
		seen := make(map[string]struct{}, issueCount)

		for i := 0; i < issueCount; i++ {
			number, err := generateTrackingNumber()
			if err != nil {
				t.Fatalf("generateTrackingNumber() error = %v", err)
			}
			if _, duplicate := seen[number]; duplicate {
				t.Fatalf("duplicate tracking number %q on call %d", number, i+1)
			}
			seen[number] = struct{}{}
		}
	})

	t.Run("suffix is not derived from the clock", func(t *testing.T) {
		// A clock-derived suffix advances monotonically within a day. Random
		// codes should not, so a run of samples must not come out sorted.
		const sampleCount = 32
		codes := make([]string, sampleCount)

		for i := range codes {
			number, err := generateTrackingNumber()
			if err != nil {
				t.Fatalf("generateTrackingNumber() error = %v", err)
			}
			codes[i] = number[strings.LastIndex(number, "-")+1:]
		}

		ascending := true
		for i := 1; i < len(codes); i++ {
			if codes[i] <= codes[i-1] {
				ascending = false
				break
			}
		}
		if ascending {
			t.Error("codes came out in ascending order, suggesting they are derived from the clock rather than random")
		}
	})
}
