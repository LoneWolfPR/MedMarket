package inbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// QuoteView contains the shape of a final price quote
type QuoteView struct {
	OfferID      uuid.UUID
	Quote        pharmacy.PriceQuote
	PharmacyName string
	Total        shared.Money
}

// PriceSearchService defines the methods the service must implement
type PriceSearchService interface {
	GetQuoteList(ctx context.Context, userID, rxID uuid.UUID) ([]QuoteView, error)
}
