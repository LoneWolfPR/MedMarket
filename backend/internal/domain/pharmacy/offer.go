package pharmacy

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Offer is the holder of persistent information about a quote for a prescription
type Offer struct {
	ID             uuid.UUID
	Quote          PriceQuote
	ExpiresAt      time.Time
	PrescriptionID uuid.UUID
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingQuote  = errors.New("quote is missing")
	ErrInvalidExpiry = errors.New("expires at is invalid")
	ErrMissingRXID   = errors.New("prescription id is missing")
)

// NewOfferParams are the input params necessary to construct an instance
type NewOfferParams struct {
	Quote          PriceQuote
	ExpiresAt      time.Time
	PrescriptionID uuid.UUID
}

// NewOffer constructs an instance
func NewOffer(p NewOfferParams) (*Offer, error) {
	if p.Quote.IsZero() {
		return nil, ErrMissingQuote
	}
	if p.ExpiresAt.IsZero() {
		return nil, ErrInvalidExpiry
	}
	if p.PrescriptionID == uuid.Nil {
		return nil, ErrMissingRXID
	}
	return &Offer{
		Quote:          p.Quote,
		ExpiresAt:      p.ExpiresAt,
		PrescriptionID: p.PrescriptionID,
	}, nil
}
