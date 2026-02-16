package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	pin := "1234"
	bytes, _ := bcrypt.GenerateFromPassword([]byte(pin), 10)
	fmt.Println(string(bytes))
}
