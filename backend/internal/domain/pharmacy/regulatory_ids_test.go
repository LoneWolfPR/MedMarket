package pharmacy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
)

func TestNewNPI_Valid(t *testing.T) {
	// Both pass the 80840-prefixed Luhn check (sums to a multiple of 10).
	tests := map[string]string{
		"canonical valid": "1234567893",
		"second valid":    "1245319599",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewNPI(raw)

			require.NoError(t, err)
			assert.Equal(t, raw, got.String(), "stored value is the raw NPI")
			assert.False(t, got.IsZero())
		})
	}
}

func TestNewNPI_Invalid(t *testing.T) {
	tests := map[string]string{
		"empty":                  "",
		"too short":              "123456789",
		"too long":               "12345678901",
		"non-digit":              "123456789a",
		"fails luhn (last +1)":   "1234567894",
		"fails luhn (zero tail)": "1234567890",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewNPI(raw)

			require.ErrorIs(t, err, pharmacy.ErrInvalidNPI)
			assert.True(t, got.IsZero(), "expected zero NPI on error")
		})
	}
}

func TestNewDEA_Valid(t *testing.T) {
	// digits 1234563: (1+3+5) + 2*(2+4+6) = 33, 33 % 10 == 3 == check digit.
	tests := map[string]string{
		"uppercase letters":            "AF1234563",
		"lowercase accepted (no norm)": "af1234563",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewDEA(raw)

			require.NoError(t, err)
			assert.Equal(t, raw, got.String(), "stored as-is; letters are not upper-cased")
			assert.False(t, got.IsZero())
		})
	}
}

func TestNewDEA_Invalid(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"too short":        "AF123456",
		"too long":         "AF12345678",
		"digit in letters": "1F1234563",
		"letter in digits": "AFA234563",
		"bad check digit":  "AF1234560",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewDEA(raw)

			require.ErrorIs(t, err, pharmacy.ErrInvalidDEA)
			assert.True(t, got.IsZero(), "expected zero DEA on error")
		})
	}
}

func TestNewNCPDP_Valid(t *testing.T) {
	tests := map[string]string{
		"seven digits":      "1234567",
		"all zeros allowed": "0000000",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewNCPDP(raw)

			require.NoError(t, err)
			assert.Equal(t, raw, got.String())
			assert.False(t, got.IsZero())
		})
	}
}

func TestNewNCPDP_Invalid(t *testing.T) {
	tests := map[string]string{
		"empty":          "",
		"too short":      "123456",
		"too long":       "12345678",
		"non-digit tail": "123456A",
		"all letters":    "abcdefg",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pharmacy.NewNCPDP(raw)

			require.ErrorIs(t, err, pharmacy.ErrInvalidNCPDP)
			assert.True(t, got.IsZero(), "expected zero NCPDP on error")
		})
	}
}

// TestRegulatoryID_ZeroValues documents that the zero value of each VO is
// IsZero and stringifies empty (used by NewPharmacy's missing-field guards).
func TestRegulatoryID_ZeroValues(t *testing.T) {
	var (
		npi   pharmacy.NPI
		dea   pharmacy.DEA
		ncpdp pharmacy.NCPDP
	)

	assert.True(t, npi.IsZero())
	assert.Empty(t, npi.String())
	assert.True(t, dea.IsZero())
	assert.Empty(t, dea.String())
	assert.True(t, ncpdp.IsZero())
	assert.Empty(t, ncpdp.String())
}
