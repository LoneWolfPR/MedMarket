package app_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

type fakeShippingSignaler struct {
	signalFn func(ctx context.Context, orderID uuid.UUID, update outbound.ShippingUpdate) error
}

func (f fakeShippingSignaler) SignalShipping(
	ctx context.Context, orderID uuid.UUID, update outbound.ShippingUpdate,
) error {
	return f.signalFn(ctx, orderID, update)
}

var _ outbound.ShippingSignaler = fakeShippingSignaler{}

func newShippingService(t *testing.T, signaler outbound.ShippingSignaler) *app.ShippingService {
	t.Helper()
	svc, err := app.NewShippingService(app.NewShippingServiceParams{
		Logger:   slog.New(slog.DiscardHandler),
		Signaler: signaler,
	})
	require.NoError(t, err)
	return svc
}

func inboundUpdate() inbound.ShippingUpdate {
	return inbound.ShippingUpdate{
		Status:     "picked_up",
		TrackingID: "trk-1",
		OccurredAt: time.Now().UTC(),
	}
}

func TestNewShippingService_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	tests := map[string]app.NewShippingServiceParams{
		"missing logger":   {Signaler: fakeShippingSignaler{}},
		"missing signaler": {Logger: logger},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := app.NewShippingService(params)
			require.Error(t, err)
		})
	}
}

func TestShippingService_SendShippingUpdate_HappyPath(t *testing.T) {
	orderID := uuid.New()
	update := inboundUpdate()

	var gotOrderID uuid.UUID
	var gotUpdate outbound.ShippingUpdate
	signaler := fakeShippingSignaler{
		signalFn: func(_ context.Context, oid uuid.UUID, u outbound.ShippingUpdate) error {
			gotOrderID, gotUpdate = oid, u
			return nil
		},
	}

	err := newShippingService(t, signaler).SendShippingUpdate(context.Background(), orderID, update)
	require.NoError(t, err)

	// The order id and the inbound update are translated onto the outbound DTO.
	assert.Equal(t, orderID, gotOrderID)
	assert.Equal(t, update.Status, gotUpdate.Status)
	assert.Equal(t, update.TrackingID, gotUpdate.TrackingID)
	assert.Equal(t, update.OccurredAt, gotUpdate.OccurredAt)
}

func TestShippingService_SendShippingUpdate_NotFoundMapsToInboundSentinel(t *testing.T) {
	signaler := fakeShippingSignaler{
		signalFn: func(context.Context, uuid.UUID, outbound.ShippingUpdate) error {
			return outbound.ErrOrderWorkflowNotFound
		},
	}

	err := newShippingService(t, signaler).SendShippingUpdate(context.Background(), uuid.New(), inboundUpdate())
	require.ErrorIs(t, err, inbound.ErrOrderWorkflowNotFound)
}

func TestShippingService_SendShippingUpdate_OtherErrorIsGeneric(t *testing.T) {
	// An unexpected signaling failure must surface (not be swallowed as success),
	// and it is not the typed not-found 404.
	signaler := fakeShippingSignaler{
		signalFn: func(context.Context, uuid.UUID, outbound.ShippingUpdate) error { return errBoom },
	}

	err := newShippingService(t, signaler).SendShippingUpdate(context.Background(), uuid.New(), inboundUpdate())
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrOrderWorkflowNotFound)
}
