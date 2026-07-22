package inbound

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// OrderView is the shape of the response from placeing an order
type OrderView struct {
	OrderID  uuid.UUID
	ItemName string
	Qty      int
	Status   string
}

//nolint:revive // sentinel error
var (
	ErrOfferExpired       = errors.New("offer is expired")
	ErrOfferNotFound      = errors.New("offer not found")
	ErrOrderAlreadyPlaced = errors.New("order already placed")
	ErrOrderNotFound      = errors.New("order not found")
)

// OrderInput holds the input params needed run the place order adapter
type OrderInput struct {
	UserID        uuid.UUID
	OfferID       uuid.UUID
	PaymentMethod string
}

// OrderStatusView represents the values returned when fetching the order status
type OrderStatusView struct {
	OrderID        uuid.UUID
	Status         string
	ShippingStatus string
}

// OrderService defines the methods the service needs to implement
type OrderService interface {
	PlaceOrder(ctx context.Context, i OrderInput) (OrderView, error)
	GetOrderStatus(ctx context.Context, userID, orderID uuid.UUID) (OrderStatusView, error)
}
