package outbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
)

// PharmacyClient declares all the methods the adapter must implement
type PharmacyClient interface {
	Search(ctx context.Context, criteria pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error)
}
