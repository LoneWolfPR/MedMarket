package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

func TestNewMoneyFromCents_Valid(t *testing.T) {
	tests := map[string]int64{
		"zero":     0,
		"positive": 1299,
		"large":    100000000,
	}

	for name, cents := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := shared.NewMoneyFromCents(cents)

			require.NoError(t, err)
			assert.Equal(t, cents, got.Cents())
		})
	}
}

func TestNewMoneyFromCents_Negative(t *testing.T) {
	got, err := shared.NewMoneyFromCents(-1)

	require.ErrorIs(t, err, shared.ErrInvalidMoneyValue)
	assert.True(t, got.IsZero(), "expected zero Money on error")
}

func TestNewMoneyFromDollars_Valid(t *testing.T) {
	tests := map[string]struct {
		dollars   string
		wantCents int64
	}{
		"dollars and cents":      {dollars: "12.99", wantCents: 1299},
		"whole dollars no point": {dollars: "12", wantCents: 1200},
		"cents only":             {dollars: "0.05", wantCents: 5},
		"zero":                   {dollars: "0", wantCents: 0},
		"hundreds":               {dollars: "100", wantCents: 10000},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := shared.NewMoneyFromDollars(tc.dollars)

			require.NoError(t, err)
			assert.Equal(t, tc.wantCents, got.Cents())
		})
	}
}

func TestNewMoneyFromDollars_Invalid(t *testing.T) {
	// The regex requires digits and, if present, exactly two decimal places.
	tests := map[string]string{
		"one decimal place":    "12.9",
		"three decimal places": "12.999",
		"no leading digit":     ".99",
		"negative":             "-1",
		"empty":                "",
		"currency symbol":      "$12.99",
		"thousands separator":  "1,299.00",
		"trailing space":       "12.99 ",
	}

	for name, dollars := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := shared.NewMoneyFromDollars(dollars)

			require.ErrorIs(t, err, shared.ErrInvalidDollarString)
			assert.True(t, got.IsZero(), "expected zero Money on error")
		})
	}
}

func TestMoney_IsZero(t *testing.T) {
	zero, err := shared.NewMoneyFromCents(0)
	require.NoError(t, err)
	assert.True(t, zero.IsZero())

	nonZero, err := shared.NewMoneyFromCents(1)
	require.NoError(t, err)
	assert.False(t, nonZero.IsZero())
}

func TestMoney_String(t *testing.T) {
	tests := map[string]struct {
		cents int64
		want  string
	}{
		"whole dollars":       {cents: 100, want: "$1.00"},
		"dollars and cents":   {cents: 1299, want: "$12.99"},
		"cents only":          {cents: 5, want: "$0.05"},
		"zero":                {cents: 0, want: "$0.00"},
		"padded single digit": {cents: 1009, want: "$10.09"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, err := shared.NewMoneyFromCents(tc.cents)
			require.NoError(t, err)

			assert.Equal(t, tc.want, m.String())
		})
	}
}
