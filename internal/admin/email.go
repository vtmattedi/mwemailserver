package admin

import (
	"fmt"
	"net/mail"
	"strings"
)

func NormalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", fmt.Errorf("email is required")
	}

	address, err := mail.ParseAddress(value)
	if err != nil || strings.ToLower(address.Address) != value {
		return "", fmt.Errorf("invalid email address")
	}

	return value, nil
}
