package hash

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPassword(t *testing.T) {
	t.Run("generates valid bcrypt hash", func(t *testing.T) {
		password := "mysecretpassword"
		hash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if hash == "" {
			t.Fatal("Password() returned empty hash")
		}
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Errorf("Password() hash does not have bcrypt prefix: %s", hash)
		}
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		if err != nil {
			t.Errorf("Generated hash is not valid for password: %v", err)
		}
	})

	t.Run("different passwords generate different hashes", func(t *testing.T) {
		password1 := "password1"
		password2 := "password2"
		hash1, err := Password(password1)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		hash2, err := Password(password2)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if hash1 == hash2 {
			t.Error("Password() generated same hash for different passwords")
		}
	})

	t.Run("same password generates different hashes due to salt", func(t *testing.T) {
		password := "samepassword"
		hash1, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		hash2, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if hash1 == hash2 {
			t.Error("Password() generated same hash for same password (expected different salts)")
		}
	})

	t.Run("hash is not equal to original password", func(t *testing.T) {
		password := "mysecretpassword"
		hash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if hash == password {
			t.Error("Password() returned hash equal to original password")
		}
	})
}

func TestCheckPassword(t *testing.T) {
	t.Run("correct password matches hash", func(t *testing.T) {
		password := "correctpassword"
		hash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if !CheckPassword(password, hash) {
			t.Error("CheckPassword() returned false for correct password")
		}
	})

	t.Run("incorrect password does not match hash", func(t *testing.T) {
		password := "correctpassword"
		wrongPassword := "wrongpassword"
		hash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		if CheckPassword(wrongPassword, hash) {
			t.Error("CheckPassword() returned true for incorrect password")
		}
	})

	t.Run("empty password works correctly", func(t *testing.T) {
		password := ""
		hash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error for empty password: %v", err)
		}
		if !CheckPassword("", hash) {
			t.Error("CheckPassword() returned false for correct empty password")
		}
		if CheckPassword("nonempty", hash) {
			t.Error("CheckPassword() returned true for incorrect password with empty hash")
		}
	})
}

func TestPIN(t *testing.T) {
	t.Run("generates valid bcrypt hash for PIN", func(t *testing.T) {
		pin := "1234"
		hash, err := PIN(pin)
		if err != nil {
			t.Fatalf("PIN() returned error: %v", err)
		}
		if hash == "" {
			t.Fatal("PIN() returned empty hash")
		}
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Errorf("PIN() hash does not have bcrypt prefix: %s", hash)
		}
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
		if err != nil {
			t.Errorf("Generated hash is not valid for PIN: %v", err)
		}
	})

	t.Run("PIN cost is used (different from password cost)", func(t *testing.T) {
		password := "testvalue"
		pin := "testvalue"
		passwordHash, err := Password(password)
		if err != nil {
			t.Fatalf("Password() returned error: %v", err)
		}
		pinHash, err := PIN(pin)
		if err != nil {
			t.Fatalf("PIN() returned error: %v", err)
		}
		passwordCost, err := bcrypt.Cost([]byte(passwordHash))
		if err != nil {
			t.Fatalf("Failed to get password cost: %v", err)
		}
		pinCost, err := bcrypt.Cost([]byte(pinHash))
		if err != nil {
			t.Fatalf("Failed to get PIN cost: %v", err)
		}
		if passwordCost != DefaultCost {
			t.Errorf("Password cost = %d, want %d", passwordCost, DefaultCost)
		}
		if pinCost != PINCost {
			t.Errorf("PIN cost = %d, want %d", pinCost, PINCost)
		}
		if passwordCost == pinCost {
			t.Errorf("Password cost (%d) should be different from PIN cost (%d)", passwordCost, pinCost)
		}
	})
}

func TestCheckPIN(t *testing.T) {
	t.Run("correct PIN matches hash", func(t *testing.T) {
		pin := "1234"
		hash, err := PIN(pin)
		if err != nil {
			t.Fatalf("PIN() returned error: %v", err)
		}
		if !CheckPIN(pin, hash) {
			t.Error("CheckPIN() returned false for correct PIN")
		}
	})

	t.Run("incorrect PIN does not match hash", func(t *testing.T) {
		pin := "1234"
		wrongPin := "5678"
		hash, err := PIN(pin)
		if err != nil {
			t.Fatalf("PIN() returned error: %v", err)
		}
		if CheckPIN(wrongPin, hash) {
			t.Error("CheckPIN() returned true for incorrect PIN")
		}
	})
}
