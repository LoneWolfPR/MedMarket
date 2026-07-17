package pharmacy

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// OrderInput is VO type that represents all the information
// necessary for a medication order to be placed
type OrderInput struct {
	idempotencyKey  string
	pharmacyItemID  string
	qty             int
	shippingAddress shared.Address
	recipientName   string
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingIdempotencyKey  = errors.New("idempotency key is missing")
	ErrMissingPharmItemID     = errors.New("pharmacy item id is missing")
	ErrInvalidQty             = errors.New("invalid quantity")
	ErrInvalidShippingAddress = errors.New("invalid shipping address")
	ErrMissingRecipientName   = errors.New("recipient name is missing")
)

// NewOrderInputParams holds the inputs necessary to construct an instance
type NewOrderInputParams struct {
	IdempotencyKey  string
	PharmacyItemID  string
	Qty             int
	ShippingAddress shared.Address
	RecipientName   string
}

// NewOrderInput constructs and instance
func NewOrderInput(p NewOrderInputParams) (OrderInput, error) {
	trimmedKey := strings.TrimSpace(p.IdempotencyKey)
	trimmedPharmItemID := strings.TrimSpace(p.PharmacyItemID)
	trimmedName := strings.TrimSpace(p.RecipientName)
	if trimmedKey == "" {
		return OrderInput{}, ErrMissingIdempotencyKey
	}
	if trimmedPharmItemID == "" {
		return OrderInput{}, ErrMissingPharmItemID
	}
	if p.Qty <= 0 {
		return OrderInput{}, ErrInvalidQty
	}
	if !p.ShippingAddress.IsValid() {
		return OrderInput{}, ErrInvalidShippingAddress
	}
	if trimmedName == "" {
		return OrderInput{}, ErrMissingRecipientName
	}
	return OrderInput{
		idempotencyKey:  trimmedKey,
		pharmacyItemID:  trimmedPharmItemID,
		qty:             p.Qty,
		shippingAddress: p.ShippingAddress,
		recipientName:   trimmedName,
	}, nil
}

// IdempotencyKey is a getter
func (i OrderInput) IdempotencyKey() string {
	return i.idempotencyKey
}

// PharmacyItemID is a getter
func (i OrderInput) PharmacyItemID() string {
	return i.pharmacyItemID
}

// Qty is a getter
func (i OrderInput) Qty() int {
	return i.qty
}

// ShippingAddress is a getter
func (i OrderInput) ShippingAddress() shared.Address {
	return i.shippingAddress
}

// RecipientName is a getter
func (i OrderInput) RecipientName() string {
	return i.recipientName
}
