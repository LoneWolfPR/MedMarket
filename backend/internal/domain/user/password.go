package user

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Password is a value type object
type Password struct {
	value string
}

//nolint:revive // constants are self-documenting
const (
	MinLength           = 8
	MaxLength           = 32
	AllowedSpecialChars = "`!@#$%^&*()_-+=[]{}|\\:;\"'<,>.?/~"
)

//nolint:revive // sentinal errors are self-documenting
var (
	ErrInvalidPasswordLength = errors.New("invalid password length: must be 8-32 characters")
	ErrInvalidCharacter      = fmt.Errorf("invalid character in password only letters, numbers, and %s are allowed", AllowedSpecialChars)
	ErrInvalidPassword       = fmt.Errorf("invalid password: it must be 8-32 characters and contain an upper and lower case letter, a number, and a special character")
)

// NewPassword is the constructor for creating and validating a new password
func NewPassword(pw string) (Password, error) {
	length := utf8.RuneCountInString(pw)

	// validate length and contents
	if length < MinLength || length > MaxLength {
		return Password{}, ErrInvalidPasswordLength
	}
	containsUpper := false
	containsLower := false
	containsNumber := false
	containsSpecial := false

	for _, runeValue := range pw {
		switch {
		case unicode.IsUpper(runeValue):
			containsUpper = true
		case unicode.IsLower(runeValue):
			containsLower = true
		case unicode.IsDigit(runeValue):
			containsNumber = true
		case strings.ContainsRune(AllowedSpecialChars, runeValue):
			containsSpecial = true
		default:
			return Password{}, ErrInvalidCharacter
		}
	}

	if containsUpper && containsLower && containsNumber && containsSpecial {
		return Password{value: pw}, nil
	}
	return Password{}, ErrInvalidPassword
}

// IsZero checks for an empty password
func (p Password) IsZero() bool { return p.value == "" }

func (p Password) String() string { return p.value }
