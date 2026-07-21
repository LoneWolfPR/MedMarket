package order

import (
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// PlacementRequest holds the values necessary to start an
// order placement workflow
type PlacementRequest struct {
	orderID        uuid.UUID
	pharmacyCode   string
	pharmacyItemID string
	itemName       string
	recipientName  string
	recipientEmail string
	qty            int
	address        shared.Address
	amountCents    int64
	paymentMethod  string
}

// NewPlacementRequestParams holds the parameters necessary for constructing an instance
type NewPlacementRequestParams struct {
	OrderID        uuid.UUID
	PharmacyCode   string
	PharmacyItemID string
	ItemName       string
	RecipientName  string
	RecipientEmail string
	Qty            int
	Address        shared.Address
	AmountCents    int64
	PaymentMethod  string
}

// NewPlacementRequest constructs an instance of the type
func NewPlacementRequest(p NewPlacementRequestParams) (PlacementRequest, error) {
	if p.OrderID == uuid.Nil {
		return PlacementRequest{}, errors.New("order id is missing")
	}
	if p.PharmacyCode == "" {
		return PlacementRequest{}, errors.New("pharmacy code is missing")
	}
	if p.PharmacyItemID == "" {
		return PlacementRequest{}, errors.New("pharmacy item id is missing")
	}
	if p.ItemName == "" {
		return PlacementRequest{}, errors.New("item name is missing")
	}
	if p.RecipientName == "" {
		return PlacementRequest{}, errors.New("recipient name is missing")
	}
	if p.RecipientEmail == "" {
		return PlacementRequest{}, errors.New("recipient email is missing")
	}
	if p.Qty <= 0 {
		return PlacementRequest{}, errors.New("invalid quantity")
	}
	if !p.Address.IsValid() {
		return PlacementRequest{}, errors.New("invalid address")
	}
	if p.AmountCents < 0 {
		return PlacementRequest{}, errors.New("invalid amount")
	}
	if p.PaymentMethod == "" {
		return PlacementRequest{}, errors.New("payment method is missing")
	}
	return PlacementRequest{
		orderID:        p.OrderID,
		pharmacyCode:   p.PharmacyCode,
		pharmacyItemID: p.PharmacyItemID,
		itemName:       p.ItemName,
		recipientName:  p.RecipientName,
		recipientEmail: p.RecipientEmail,
		qty:            p.Qty,
		address:        p.Address,
		amountCents:    p.AmountCents,
		paymentMethod:  p.PaymentMethod,
	}, nil
}

// OrderID is a getter
func (r PlacementRequest) OrderID() uuid.UUID {
	return r.orderID
}

// PharmacyCode is a getter
func (r PlacementRequest) PharmacyCode() string {
	return r.pharmacyCode
}

// PharmacyItemID is a getter
func (r PlacementRequest) PharmacyItemID() string {
	return r.pharmacyItemID
}

// ItemName is a getter
func (r PlacementRequest) ItemName() string {
	return r.itemName
}

// RecipientName is a getter
func (r PlacementRequest) RecipientName() string {
	return r.recipientName
}

// RecipientEmail is a getter
func (r PlacementRequest) RecipientEmail() string {
	return r.recipientEmail
}

// Qty is a getter
func (r PlacementRequest) Qty() int {
	return r.qty
}

// Address is a getter
func (r PlacementRequest) Address() shared.Address {
	return r.address
}

// AmountCents is a getter
func (r PlacementRequest) AmountCents() int64 {
	return r.amountCents
}

// PaymentMethod is a getter
func (r PlacementRequest) PaymentMethod() string {
	return r.paymentMethod
}
