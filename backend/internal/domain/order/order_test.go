package order_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
)

func TestNewOrder_Valid(t *testing.T) {
	rxID, offerID := uuid.New(), uuid.New()

	got, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: rxID,
		OfferID:        offerID,
		Qty:            30,
	})

	require.NoError(t, err)
	assert.Equal(t, rxID, got.PrescriptionID)
	assert.Equal(t, offerID, got.OfferID)
	assert.Equal(t, 30, got.Qty)
	assert.Equal(t, uuid.Nil, got.ID, "the DB assigns the id")
}

// TestNewOrder_StartsPlacedAndUnpaid pins the two facts NewOrder asserts about a
// brand-new order: it is at the start of the lifecycle, and nothing has been paid.
// Status is deliberately not a parameter — an order further along is reconstructed
// from a row, never conjured by the constructor.
func TestNewOrder_StartsPlacedAndUnpaid(t *testing.T) {
	got, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: uuid.New(),
		OfferID:        uuid.New(),
		Qty:            1,
	})

	require.NoError(t, err)
	assert.Equal(t, order.StatusPlaced, got.Status)
	assert.Nil(t, got.PricePaid, "capture has not happened yet")
	assert.Empty(t, got.PharmacyOrderID, "the pharmacy has not been reached yet")
	assert.Empty(t, got.TrackingID)
}

func TestNewOrder_Invalid(t *testing.T) {
	tests := map[string]struct {
		params  order.NewOrderParams
		wantErr error
	}{
		"missing prescription id": {
			params:  order.NewOrderParams{PrescriptionID: uuid.Nil, OfferID: uuid.New(), Qty: 1},
			wantErr: order.ErrMissingRXID,
		},
		"missing offer id": {
			params:  order.NewOrderParams{PrescriptionID: uuid.New(), OfferID: uuid.Nil, Qty: 1},
			wantErr: order.ErrMissingOfferID,
		},
		"zero quantity": {
			params:  order.NewOrderParams{PrescriptionID: uuid.New(), OfferID: uuid.New(), Qty: 0},
			wantErr: order.ErrInvalidQty,
		},
		"negative quantity": {
			params:  order.NewOrderParams{PrescriptionID: uuid.New(), OfferID: uuid.New(), Qty: -1},
			wantErr: order.ErrInvalidQty,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := order.NewOrder(tc.params)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, got)
		})
	}
}

// TestAcceptedStatuses_MatchesConstants guards the list the Ent schema renders its
// status CHECK from: a constant added without being listed here would silently be
// legal in the domain and rejected by the database.
func TestAcceptedStatuses_MatchesConstants(t *testing.T) {
	assert.ElementsMatch(t, []string{
		order.StatusPlaced,
		order.StatusConfirmed,
		order.StatusShipped,
		order.StatusDelivered,
		order.StatusFailed,
		order.StatusCanceled,
	}, order.AcceptedStatuses)
}
