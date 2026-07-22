package outbound

import (
	"context"

	"github.com/google/uuid"
)

// OrderStatusQuerier defines the methods for the adapter to implement
type OrderStatusQuerier interface {
	QueryShippingStatus(ctx context.Context, orderID uuid.UUID) (string, error)
}
