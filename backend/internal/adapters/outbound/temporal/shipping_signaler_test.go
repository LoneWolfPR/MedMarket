package temporal_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/temporal"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	orderwf "github.com/LoneWolfPR/MedMarket/backend/workflows/order"
)

// fakeSignalClient embeds client.Client so it satisfies the whole interface while
// implementing only SignalWorkflow. Any other method the adapter calls hits the
// nil embedded interface and panics, surfacing as a test failure.
type fakeSignalClient struct {
	temporalclient.Client
	signalFn func(ctx context.Context, workflowID, runID, signalName string, arg any) error
}

func (f fakeSignalClient) SignalWorkflow(
	ctx context.Context, workflowID, runID, signalName string, arg any,
) error {
	return f.signalFn(ctx, workflowID, runID, signalName, arg)
}

func newSignaler(t *testing.T, c temporalclient.Client) *temporal.ShippingSignaler {
	t.Helper()
	s, err := temporal.NewShippingSignaler(temporal.NewShippingSignalerParams{
		Client: c,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return s
}

func shippingUpdate() outbound.ShippingUpdate {
	return outbound.ShippingUpdate{
		Status:     "picked_up",
		TrackingID: "trk-1",
		OccurredAt: time.Now().UTC(),
	}
}

func TestNewShippingSignaler_Validation(t *testing.T) {
	tests := map[string]temporal.NewShippingSignalerParams{
		"missing client": {Logger: slog.New(slog.DiscardHandler)},
		"missing logger": {Client: fakeSignalClient{}},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			s, err := temporal.NewShippingSignaler(params)
			require.Error(t, err)
			assert.Nil(t, s)
		})
	}
}

func TestSignalShipping_SendsTranslatedSignal(t *testing.T) {
	orderID := uuid.New()
	update := shippingUpdate()

	var gotWorkflowID, gotSignalName string
	var gotArg any
	c := fakeSignalClient{
		signalFn: func(_ context.Context, workflowID, _, signalName string, arg any) error {
			gotWorkflowID, gotSignalName, gotArg = workflowID, signalName, arg
			return nil
		},
	}

	err := newSignaler(t, c).SignalShipping(context.Background(), orderID, update)
	require.NoError(t, err)

	// Addressed at the order's workflow, on the agreed signal channel.
	assert.Equal(t, orderwf.OrderWorkflowID(orderID), gotWorkflowID)
	assert.Equal(t, orderwf.ShippingSignalName, gotSignalName)

	// The outbound DTO is rebuilt into the workflow's SignalPayload for the wire.
	payload, ok := gotArg.(orderwf.SignalPayload)
	require.True(t, ok, "signal arg should be a SignalPayload")
	assert.Equal(t, orderwf.ShippingStatus("picked_up"), payload.Status)
	assert.Equal(t, "trk-1", payload.TrackingID)
	assert.Equal(t, update.OccurredAt, payload.OccurredAt)
}

func TestSignalShipping_NotFoundMapsToSentinel(t *testing.T) {
	// Signaling a closed/aged-out or unknown workflow comes back as a Temporal
	// NotFound, which the adapter translates so the service can render a 404.
	c := fakeSignalClient{
		signalFn: func(context.Context, string, string, string, any) error {
			return serviceerror.NewNotFound("workflow not found")
		},
	}

	err := newSignaler(t, c).SignalShipping(context.Background(), uuid.New(), shippingUpdate())
	require.ErrorIs(t, err, outbound.ErrOrderWorkflowNotFound)
}

func TestSignalShipping_OtherErrorSurfaces(t *testing.T) {
	c := fakeSignalClient{
		signalFn: func(context.Context, string, string, string, any) error { return errBoom },
	}

	err := newSignaler(t, c).SignalShipping(context.Background(), uuid.New(), shippingUpdate())
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, outbound.ErrOrderWorkflowNotFound)
}
