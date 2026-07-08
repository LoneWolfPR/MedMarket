package shared

import (
	"errors"
	"regexp"
	"unicode"
)

// Phone is a value type object
type Phone struct {
	value string
}

var (
	phoneFormat1 = regexp.MustCompile(`^\(\d{3}\) ?\d{3}-\d{4}$`)
	phoneFormat2 = regexp.MustCompile(`^\d{3}-\d{3}-\d{4}$`)
	phoneFormat3 = regexp.MustCompile(`^\d{3}\.\d{3}\.\d{4}$`)
	phoneFormat4 = regexp.MustCompile(`^\d{10}$`)
)

// ErrInvalidPhone is a sentinel error
var ErrInvalidPhone = errors.New("invalid phone number")

// NewPhone creates a value type object for the phone field in a user profile
func NewPhone(raw string) (Phone, error) {
	// First check if empty and just return
	if raw == "" {
		return Phone{value: raw}, nil
	}

	// normalize by removing various formatting punctuation and reducing
	// to a 10 character, numeric string
	normalizedPhone, err := normalizePhone(raw)
	if err != nil {
		return Phone{}, err
	}
	return Phone{value: normalizedPhone}, nil
}

func normalizePhone(raw string) (string, error) {
	formats := []*regexp.Regexp{
		phoneFormat1,
		phoneFormat2,
		phoneFormat3,
		phoneFormat4,
	}

	validFormat := false
	for _, format := range formats {
		if format.MatchString(raw) {
			validFormat = true
			break
		}
	}

	normalized := ""
	if validFormat {
		// reduce to a numeric string
		for _, runeValue := range raw {
			if unicode.IsDigit(runeValue) {
				normalized += string(runeValue)
			}
		}
		return normalized, nil
	}
	return "", ErrInvalidPhone
}

// IsZero checks for an empty phone
func (p Phone) IsZero() bool { return p.value == "" }

func (p Phone) String() string { return p.value }
