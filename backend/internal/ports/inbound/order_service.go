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
)

// OrderService defines the methods the service needs to implement
type OrderService interface {
	PlaceOrder(ctx context.Context, userID, offerID uuid.UUID) (OrderView, error)
}
