package numhelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/numhelpers"
)

func TestNormalizeNumberString_Valid(t *testing.T) {
	// input -> canonical output
	tests := map[string]struct {
		in   string
		want string
	}{
		"whole number kept intact":     {in: "500", want: "500"},
		"trailing zeros before int":    {in: "100", want: "100"},
		"trailing fractional zero":     {in: "2.50", want: "2.5"},
		"whole-valued decimal":         {in: "2.0", want: "2"},
		"sub-one keeps leading zero":   {in: "0.5", want: "0.5"},
		"bare-dot fraction":            {in: ".5", want: "0.5"},
		"leading zeros stripped":       {in: "00005", want: "5"},
		"signed with padding":          {in: "-0000.5", want: "-0.5"},
		"scientific notation expanded": {in: "1e3", want: "1000"},
		"all zeros collapse":           {in: "0000", want: "0"},
		"zero":                         {in: "0", want: "0"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := numhelpers.NormalizeNumberString(tc.in)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeNumberString_Invalid(t *testing.T) {
	tests := map[string]string{
		"empty":              "",
		"letters":            "abc",
		"two decimals":       "1.2.3",
		"unit suffix":        "5mg",
		"trailing letter":    "12x",
		"surrounding spaces": " 5 ",
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := numhelpers.NormalizeNumberString(in)

			require.ErrorIs(t, err, numhelpers.ErrInvalidNumberString)
			assert.Empty(t, got)
		})
	}
}
