package service

import "testing"

func TestCheckPIN(t *testing.T) {
	for _, ok := range []string{"2580", "13579", "904172"} {
		if err := checkPIN(ok); err != nil {
			t.Errorf("checkPIN(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "123", "1234567", "1111", "1234", "4321", "123456", "12a4"} {
		if err := checkPIN(bad); err == nil {
			t.Errorf("checkPIN(%q) accepted", bad)
		}
	}
}

func TestTemporaryCredentials(t *testing.T) {
	for i := 0; i < 50; i++ {
		pw := temporaryPassword()
		if len(pw) != 10 {
			t.Fatalf("temporary password %q has length %d", pw, len(pw))
		}
		if checkPassword(pw) != nil {
			t.Fatalf("temporary password %q fails policy", pw)
		}
	}
	if d := randomDigits(6); len(d) != 6 {
		t.Errorf("randomDigits = %q", d)
	}
}
