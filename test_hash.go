package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$f48u47fxlqK7SL9qt0V.6OA0EDvqe4s4mYJa7TpWhRfwaAr8K7.c." 
	token := "cm_live_ed957055_W9mpvGAT7LyOjS2oU8P1bP1MHapHRp8PeGBiCnRmuGE"
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
	if err != nil {
		fmt.Println("Hash mismatch:", err)
	} else {
		fmt.Println("Hash matches!")
	}
}
