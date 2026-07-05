package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

func TestNewEmail_Valid(t *testing.T) {
	tests := map[string]struct {
		raw  string
		want string
	}{
		"already normalized":     {raw: "user@example.com", want: "user@example.com"},
		"uppercased is lowered":  {raw: "User@Example.COM", want: "user@example.com"},
		"surrounding whitespace": {raw: "  user@example.com  ", want: "user@example.com"},
		"plus addressing":        {raw: "user+tag@example.com", want: "user+tag@example.com"},
		"subdomain":              {raw: "user@mail.example.co.uk", want: "user@mail.example.co.uk"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewEmail(tc.raw)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
			assert.False(t, got.IsZero())
		})
	}
}

func TestNewEmail_Invalid(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"whitespace only":  "   ",
		"no at sign":       "notanemail",
		"missing domain":   "user@",
		"missing local":    "@example.com",
		"double at":        "user@@example.com",
		"internal spaces":  "user name@example.com",
		"trailing garbage": "user@example.com junk",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewEmail(raw)

			require.ErrorIs(t, err, user.ErrInvalidEmail)
			assert.True(t, got.IsZero(), "expected zero Email on error")
		})
	}
}

// TestEmailZeroValue documents that the zero Email is IsZero and stringifies empty.
func TestEmailZeroValue(t *testing.T) {
	var e user.Email

	assert.True(t, e.IsZero())
	assert.Empty(t, e.String())
}
