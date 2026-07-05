package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

func TestNewPhone_ValidFormats(t *testing.T) {
	// every accepted format normalizes to the same bare 10-digit string
	const want = "5551234567"
	tests := map[string]string{
		"parenthesized with space": "(555) 123-4567",
		"parenthesized no space":   "(555)123-4567",
		"dash separated":           "555-123-4567",
		"dot separated":            "555.123.4567",
		"bare ten digits":          "5551234567",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewPhone(raw)

			require.NoError(t, err)
			assert.Equal(t, want, got.String(), "should normalize to bare digits")
			assert.False(t, got.IsZero())
		})
	}
}

// TestNewPhone_EmptyIsAllowed documents that phone is optional: an empty string
// is valid and yields a zero Phone (the service passes user input straight in).
func TestNewPhone_EmptyIsAllowed(t *testing.T) {
	got, err := user.NewPhone("")

	require.NoError(t, err)
	assert.True(t, got.IsZero())
	assert.Empty(t, got.String())
}

func TestNewPhone_Invalid(t *testing.T) {
	tests := map[string]string{
		"too few digits":         "555-123-456",
		"too many digits":        "55512345678",
		"ten letters":            "abcdefghij",
		"letters in body":        "555-abc-4567",
		"space separated":        "555 123 4567", // intentionally unsupported
		"surrounding whitespace": " 5551234567 ", // NewPhone does not trim
		"mixed separators":       "555-123.4567",
		"leading country code":   "15551234567",
		"plus country code":      "+15551234567",
		"dash after parens":      "(555)-123-4567",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewPhone(raw)

			require.ErrorIs(t, err, user.ErrInvalidPhone)
			assert.True(t, got.IsZero(), "expected zero Phone on error")
		})
	}
}

// TestPhoneZeroValue documents that the zero Phone is IsZero and stringifies empty.
func TestPhoneZeroValue(t *testing.T) {
	var p user.Phone

	assert.True(t, p.IsZero())
	assert.Empty(t, p.String())
}
