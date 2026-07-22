package inbound

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ShippingUpdate holds the properties that will be sent in as part of the update
type ShippingUpdate struct {
	Status     string
	TrackingID string
	OccurredAt time.Time
}

//nolint:revive // sentinel error
var ErrOrderWorkflowNotFound = errors.New("order workflow not found")

// ShippingService defines the methods needed to implement the service
type ShippingService interface {
	SendShippingUpdate(ctx context.Context, orderID uuid.UUID, update ShippingUpdate) error
}
