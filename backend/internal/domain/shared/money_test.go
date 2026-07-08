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
