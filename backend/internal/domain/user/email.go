package user

import (
	"errors"
	"net/mail"
	"strings"
)

// Email is a value type object
type Email struct {
	value string
}

// ErrInvalidEmail is a generic error for email validation failure
var ErrInvalidEmail = errors.New("invalid email")

// NewEmail is the constructor for creating and validating the Email type object
func NewEmail(raw string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))

	parsed, err := mail.ParseAddress(normalized)
	if err != nil {
		return Email{}, ErrInvalidEmail
	}

	return Email{value: parsed.Address}, nil
}

// IsZero checks for an empty email
func (e Email) IsZero() bool { return e.value == "" }

func (e Email) String() string { return e.value }
