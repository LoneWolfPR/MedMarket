package shared

// White-box (package shared) because MedStrength has no getters yet: asserting
// the normalized value/unit means reading the fields directly. Move to
// shared_test if accessors are added later.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMedStrength_Valid(t *testing.T) {
	tests := map[string]struct {
		value     string
		unit      string
		wantValue string
		wantUnit  string
	}{
		"trims fractional zero":  {value: "2.50", unit: "mg", wantValue: "2.5", wantUnit: "mg"},
		"whole number kept":      {value: "500", unit: "mg", wantValue: "500", wantUnit: "mg"},
		"uppercase unit lowered": {value: "10", unit: "MG", wantValue: "10", wantUnit: "mg"},
		"unit trimmed + lowered": {value: "0.5", unit: "  ML  ", wantValue: "0.5", wantUnit: "ml"},
		"micrograms":             {value: "88", unit: "mcg", wantValue: "88", wantUnit: "mcg"},
		"sub-one keeps zero":     {value: ".25", unit: "mg", wantValue: "0.25", wantUnit: "mg"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NewMedStrength(tc.value, tc.unit)

			require.NoError(t, err)
			assert.Equal(t, tc.wantValue, got.value)
			assert.Equal(t, tc.wantUnit, got.unit)
		})
	}
}

func TestNewMedStrength_InvalidValue(t *testing.T) {
	// unit is valid in every case so the value guard is what fires.
	tests := map[string]string{
		"zero":          "0",
		"all zeros":     "0000",
		"negative":      "-5",
		"negative zero": "-0",
		"non-numeric":   "abc",
		"empty":         "",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewMedStrength(value, "mg")

			require.ErrorIs(t, err, ErrInvalidMedStrengthValue)
		})
	}
}

func TestNewMedStrength_InvalidUnit(t *testing.T) {
	// value is valid in every case so the unit guard is what fires.
	tests := map[string]string{
		"substring of accepted": "m", // regression: must not match via Contains
		"single letter g":       "g",
		"unaccepted unit":       "kg",
		"empty":                 "",
		"full word":             "milligrams",
	}

	for name, unit := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewMedStrength("5", unit)

			require.ErrorIs(t, err, ErrInvalidMedStrengthUnit)
		})
	}
}
