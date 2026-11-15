package phone

import "testing"

func TestNormalize(t *testing.T) {
	ok := map[string]string{
		"0911234567":       "0911234567",
		"0711234567":       "0711234567",
		"911234567":        "0911234567",
		"+251911234567":    "0911234567",
		"251911234567":     "0911234567",
		"+251 91 123 4567": "0911234567",
		"091-123-4567":     "0911234567",
		"(091) 123.4567":   "0911234567",
	}
	for in, want := range ok {
		got, err := Normalize(in)
		if err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	bad := []string{"", "0811234567", "091123456", "09112345678", "+1 415 555 0100", "09x1234567", "1+251911234567"}
	for _, in := range bad {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q; want error", in, got)
		}
	}
}

func TestMask(t *testing.T) {
	if got := Mask("0911234567"); got != "0911••••67" {
		t.Errorf("Mask = %q", got)
	}
}
