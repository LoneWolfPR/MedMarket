package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

func TestAddress_IsValid(t *testing.T) {
	full := shared.Address{
		Street1: "123 Main St",
		Street2: "Apt 4",
		City:    "Springfield",
		State:   "IL",
		Zip:     "62704",
	}

	tests := map[string]struct {
		mutate func(a *shared.Address)
		want   bool
	}{
		"all fields present": {mutate: func(_ *shared.Address) {}, want: true},
		"street2 optional":   {mutate: func(a *shared.Address) { a.Street2 = "" }, want: true},
		"missing street1":    {mutate: func(a *shared.Address) { a.Street1 = "" }, want: false},
		"missing city":       {mutate: func(a *shared.Address) { a.City = "" }, want: false},
		"missing state":      {mutate: func(a *shared.Address) { a.State = "" }, want: false},
		"missing zip":        {mutate: func(a *shared.Address) { a.Zip = "" }, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := full
			tc.mutate(&a)

			assert.Equal(t, tc.want, a.IsValid())
		})
	}
}

// TestAddress_ZeroValue documents that the zero Address is not valid.
func TestAddress_ZeroValue(t *testing.T) {
	var a shared.Address

	assert.False(t, a.IsValid())
}
