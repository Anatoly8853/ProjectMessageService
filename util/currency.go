package util

import (
	"ProjectMessageService/config"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func IsSupportedCurrency(msg string) bool {
	for _, currencyCode := range config.MessageTypes {
		if currencyCode == msg {
			return true
		}
	}

	return false
}

// HashPassword возвращает bcrypt хэш пароля
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

// CheckPassword проверяет правильность предоставленного пароля
func CheckPassword(password string, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
