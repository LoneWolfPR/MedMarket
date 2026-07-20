package outbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
)

// OrderStarter defines the methods the adapter must implement
type OrderStarter interface {
	StartOrder(ctx context.Context, i order.PlacementRequest) error
}
