package hash

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultCost = 12
	PINCost     = 10
)

func Password(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func PIN(pin string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pin), PINCost)
	return string(bytes), err
}

func CheckPIN(pin, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
	return err == nil
}
