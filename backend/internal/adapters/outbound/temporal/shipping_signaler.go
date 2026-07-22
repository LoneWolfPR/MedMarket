package temporal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	temporalorder "github.com/LoneWolfPR/MedMarket/backend/workflows/order"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// ShippingSignaler is the adapter for signaling the order workflow with a
// shipping update
type ShippingSignaler struct {
	client client.Client
	logger *slog.Logger
}

var _ outbound.ShippingSignaler = (*ShippingSignaler)(nil)

// NewShippingSignalerParams holds the parameters necessary to construct the adapter
type NewShippingSignalerParams struct {
	Client client.Client
	Logger *slog.Logger
}

// NewShippingSignaler constructs a new instance of the adapter
func NewShippingSignaler(p NewShippingSignalerParams) (*ShippingSignaler, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &ShippingSignaler{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// SignalShipping signals a running order workflow with the update
func (s *ShippingSignaler) SignalShipping(
	ctx context.Context,
	orderID uuid.UUID,
	update outbound.ShippingUpdate,
) error {
	workflowID := temporalorder.OrderWorkflowID(orderID)
	signalPayload := temporalorder.SignalPayload{
		Status:     temporalorder.ShippingStatus(update.Status),
		TrackingID: update.TrackingID,
		OccurredAt: update.OccurredAt,
	}
	if err := s.client.SignalWorkflow(
		ctx, workflowID,
		"",
		temporalorder.ShippingSignalName,
		signalPayload,
	); err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return outbound.ErrOrderWorkflowNotFound
		}
		return fmt.Errorf("error signaling order workflow: %w", err)
	}
	return nil
}
