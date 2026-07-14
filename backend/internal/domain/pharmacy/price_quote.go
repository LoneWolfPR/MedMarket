package pharmacy

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// PriceQuote is the VO for holding information about a price from
// a pharmacy
type PriceQuote struct {
	price          shared.Money
	pharmacyID     uuid.UUID
	pharmacyItemID string // Pharmacy specific item ID (sku, etc.)
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingPharmacyID     = errors.New("pharmacy id is missing")
	ErrMissingPharmacyItemID = errors.New("pharmacy item id is missing")
)

// NewPriceQuoteParams has the input params for creating a new price quote
type NewPriceQuoteParams struct {
	Price          shared.Money
	PharmacyID     uuid.UUID
	PharmacyItemID string
}

// NewPriceQuote is the constructor for setting up a new price from
// a pharmacy
func NewPriceQuote(p NewPriceQuoteParams) (PriceQuote, error) {
	if p.PharmacyID == uuid.Nil {
		return PriceQuote{}, ErrMissingPharmacyID
	}
	trimmedItemID := strings.TrimSpace(p.PharmacyItemID)
	if trimmedItemID == "" {
		return PriceQuote{}, ErrMissingPharmacyItemID
	}

	return PriceQuote{
		price:          p.Price,
		pharmacyID:     p.PharmacyID,
		pharmacyItemID: trimmedItemID,
	}, nil
}

// Price is a getter
func (q *PriceQuote) Price() shared.Money {
	return q.price
}

// PharmacyID is a getter
func (q *PriceQuote) PharmacyID() uuid.UUID {
	return q.pharmacyID
}

// PharmacyItemID is a getter
func (q *PriceQuote) PharmacyItemID() string {
	return q.pharmacyItemID
}
