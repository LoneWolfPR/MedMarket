package pharmacy_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

func TestNewPriceQuote_Valid(t *testing.T) {
	price, err := shared.NewMoneyFromCents(1299)
	require.NoError(t, err)
	id := uuid.New()

	got, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     id,
		PharmacyItemID: "SKU-123",
	})

	require.NoError(t, err)
	assert.Equal(t, price, got.Price())
	assert.Equal(t, id, got.PharmacyID())
	assert.Equal(t, "SKU-123", got.PharmacyItemID())
}

// TestNewPriceQuote_ZeroPriceAllowed documents the deliberate decision that a
// $0.00 quote is valid (discounts / full insurance coverage).
func TestNewPriceQuote_ZeroPriceAllowed(t *testing.T) {
	var zeroPrice shared.Money // zero value == $0.00

	got, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          zeroPrice,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "SKU-1",
	})

	require.NoError(t, err)
	assert.True(t, got.Price().IsZero())
}

func TestNewPriceQuote_TrimsItemID(t *testing.T) {
	price, err := shared.NewMoneyFromCents(500)
	require.NoError(t, err)

	got, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "  SKU-123  ",
	})

	require.NoError(t, err)
	assert.Equal(t, "SKU-123", got.PharmacyItemID(), "surrounding whitespace is trimmed before storage")
}

func TestPriceQuote_IsZero(t *testing.T) {
	price, err := shared.NewMoneyFromCents(1299)
	require.NoError(t, err)

	built, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "SKU-1",
	})
	require.NoError(t, err)

	var freePrice shared.Money // zero value == $0.00
	free, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          freePrice,
		PharmacyID:     uuid.New(),
		PharmacyItemID: "SKU-FREE",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		quote pharmacy.PriceQuote
		want  bool
	}{
		"zero value": {
			quote: pharmacy.PriceQuote{},
			want:  true,
		},
		"fully built": {
			quote: built,
			want:  false,
		},
		// A $0.00 quote is legitimate (discounts / full insurance coverage) and must
		// not be mistaken for an unconstructed value. IsZero asks "was this ever
		// built?", never "is the price zero?" — a distinction that matters because
		// shared.Money.IsZero() *is* true for $0.00.
		"zero price, but built": {
			quote: free,
			want:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.quote.IsZero())
		})
	}
}

func TestNewPriceQuote_Invalid(t *testing.T) {
	price, err := shared.NewMoneyFromCents(500)
	require.NoError(t, err)

	tests := map[string]struct {
		params  pharmacy.NewPriceQuoteParams
		wantErr error
	}{
		"missing pharmacy id": {
			params:  pharmacy.NewPriceQuoteParams{Price: price, PharmacyID: uuid.Nil, PharmacyItemID: "SKU-1"},
			wantErr: pharmacy.ErrMissingPharmacyID,
		},
		"empty item id": {
			params:  pharmacy.NewPriceQuoteParams{Price: price, PharmacyID: uuid.New(), PharmacyItemID: ""},
			wantErr: pharmacy.ErrMissingPharmacyItemID,
		},
		"whitespace-only item id": {
			params:  pharmacy.NewPriceQuoteParams{Price: price, PharmacyID: uuid.New(), PharmacyItemID: "   "},
			wantErr: pharmacy.ErrMissingPharmacyItemID,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := pharmacy.NewPriceQuote(tc.params)

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
