package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// ShippingService is the service that calls the temporal signaler to update
// the order workflow
type ShippingService struct {
	logger   *slog.Logger
	signaler outbound.ShippingSignaler
}

var _ inbound.ShippingService = (*ShippingService)(nil)

// NewShippingServiceParams holds the parameters needed to construct the service
type NewShippingServiceParams struct {
	Logger   *slog.Logger
	Signaler outbound.ShippingSignaler
}

// NewShippingService constructs the service
func NewShippingService(p NewShippingServiceParams) (*ShippingService, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Signaler == nil {
		return nil, errors.New("signaler is missing")
	}
	return &ShippingService{
		logger:   p.Logger,
		signaler: p.Signaler,
	}, nil
}

// SendShippingUpdate calls the temporal signaler to update the order workflow
func (s *ShippingService) SendShippingUpdate(
	ctx context.Context,
	orderID uuid.UUID,
	update inbound.ShippingUpdate,
) error {
	signalerUpdate := outbound.ShippingUpdate{
		Status:     update.Status,
		TrackingID: update.TrackingID,
		OccurredAt: update.OccurredAt,
	}
	if err := s.signaler.SignalShipping(ctx, orderID, signalerUpdate); err != nil {
		if errors.Is(err, outbound.ErrOrderWorkflowNotFound) {
			return fmt.Errorf("%w: %w", inbound.ErrOrderWorkflowNotFound, err)
		}
		return fmt.Errorf("error signalling shipping update: %w", err)
	}
	return nil
}
