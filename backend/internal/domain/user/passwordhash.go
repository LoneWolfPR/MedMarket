package user

import (
	"errors"
	"strings"
)

// PasswordHash is a value object
type PasswordHash struct {
	value string
}

// ErrEmptyPasswordHash is returned when an empty string is
// passed for the password hash
var ErrEmptyPasswordHash = errors.New("password cannot be empty")

// ErrInvalidPasswordHash is returned when anything other than a missing hash fails validation
var ErrInvalidPasswordHash = errors.New("password hash is invalid")

// NewPasswordHash is the constructor for setting the PasswordHash type
func NewPasswordHash(hashed string) (PasswordHash, error) {
	trimmed := strings.TrimSpace(hashed)
	if trimmed != hashed {
		return PasswordHash{}, ErrInvalidPasswordHash
	}
	if hashed == "" {
		return PasswordHash{}, ErrEmptyPasswordHash
	}

	return PasswordHash{value: hashed}, nil
}

// IsZero checks for an empty passwordhash
func (h PasswordHash) IsZero() bool { return h.value == "" }

func (h PasswordHash) String() string { return h.value }
