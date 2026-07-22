package outbound

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ShippingUpdate holds all the update information for a shipment
type ShippingUpdate struct {
	Status     string
	TrackingID string
	OccurredAt time.Time
}

//nolint:revive // sentinel error
var ErrOrderWorkflowNotFound = errors.New("order workflow not found")

// ShippingSignaler defines the methods the adapter needs to implement
type ShippingSignaler interface {
	SignalShipping(ctx context.Context, orderID uuid.UUID, update ShippingUpdate) error
}
