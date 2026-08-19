package order

import (
	"errors"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// Order holds all the business information about an order entity
type Order struct {
	ID              uuid.UUID
	PrescriptionID  uuid.UUID
	OfferID         uuid.UUID
	Status          string
	PharmacyOrderID string
	TrackingID      string
	Qty             int
	PricePaid       *shared.Money
	PlacedAt        time.Time
}

//nolint:revive // self documenting
const (
	StatusPlaced    = "placed"
	StatusConfirmed = "confirmed"
	StatusShipped   = "shipped"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
)

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingRXID    = errors.New("prescription id is missing")
	ErrMissingOfferID = errors.New("offer id is missing")
	ErrInvalidQty     = errors.New("qty is invalid")
)

// AcceptedStatuses is a list of valid order statuses
var AcceptedStatuses = []string{
	StatusPlaced,
	StatusConfirmed,
	StatusShipped,
	StatusDelivered,
	StatusFailed,
	StatusCanceled,
}

// NewOrderParams holds the params necessary to construct an instance
type NewOrderParams struct {
	PrescriptionID uuid.UUID
	OfferID        uuid.UUID
	Qty            int
}

// NewOrder constructs a new instance
func NewOrder(p NewOrderParams) (*Order, error) {
	if p.PrescriptionID == uuid.Nil {
		return nil, ErrMissingRXID
	}
	if p.OfferID == uuid.Nil {
		return nil, ErrMissingOfferID
	}
	if p.Qty <= 0 {
		return nil, ErrInvalidQty
	}
	return &Order{
		PrescriptionID: p.PrescriptionID,
		OfferID:        p.OfferID,
		Status:         StatusPlaced,
		Qty:            p.Qty,
	}, nil
}
