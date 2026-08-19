package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var (
	ErrOrderNotFound = errors.New("order not found")
	ErrOrderExists   = errors.New("order already exists for offer")
)

// OrderRepository defines the methods adapters must implement
type OrderRepository interface {
	Create(ctx context.Context, o *order.Order) (*order.Order, error)
	Update(ctx context.Context, o *order.Order) (*order.Order, error)
	GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error)
	List(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error)
}
