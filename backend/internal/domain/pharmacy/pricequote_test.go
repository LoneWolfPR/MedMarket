package pharmacy

// White-box (package pharmacy) because PriceQuote has no getters yet: the only
// way to assert the stored/normalized fields is to read them directly. Switch to
// pharmacy_test if/when accessors are added (the order flow will likely need them).

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

func TestNewPriceQuote_Valid(t *testing.T) {
	price, err := shared.NewMoneyFromCents(1299)
	require.NoError(t, err)
	id := uuid.New()

	got, err := NewPriceQuote(NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     id,
		PharmacyItemID: "SKU-123",
	})

	require.NoError(t, err)
	assert.Equal(t, price, got.price)
	assert.Equal(t, id, got.pharmacyID)
	assert.Equal(t, "SKU-123", got.pharmacyItemID)
}

// TestNewPriceQuote_ZeroPriceAllowed documents the deliberate decision that a
// $0.00 quote is valid (discounts / full insurance coverage).
func TestNewPriceQuote_ZeroPriceAllowed(t *testing.T) {
	var zeroPrice shared.Money // zero value == $0.00

	got, err := NewPriceQuote(NewPriceQuoteParams{
		Price:          zeroPrice,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "SKU-1",
	})

	require.NoError(t, err)
	assert.True(t, got.price.IsZero())
}

func TestNewPriceQuote_TrimsItemID(t *testing.T) {
	price, err := shared.NewMoneyFromCents(500)
	require.NoError(t, err)

	got, err := NewPriceQuote(NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "  SKU-123  ",
	})

	require.NoError(t, err)
	assert.Equal(t, "SKU-123", got.pharmacyItemID, "surrounding whitespace is trimmed before storage")
}

func TestNewPriceQuote_Invalid(t *testing.T) {
	price, err := shared.NewMoneyFromCents(500)
	require.NoError(t, err)

	tests := map[string]struct {
		params  NewPriceQuoteParams
		wantErr error
	}{
		"missing pharmacy id": {
			params:  NewPriceQuoteParams{Price: price, PharmacyID: uuid.Nil, PharmacyItemID: "SKU-1"},
			wantErr: ErrMissingPharmacyID,
		},
		"empty item id": {
			params:  NewPriceQuoteParams{Price: price, PharmacyID: uuid.New(), PharmacyItemID: ""},
			wantErr: ErrMissingPharmacyItemID,
		},
		"whitespace-only item id": {
			params:  NewPriceQuoteParams{Price: price, PharmacyID: uuid.New(), PharmacyItemID: "   "},
			wantErr: ErrMissingPharmacyItemID,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewPriceQuote(tc.params)

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
