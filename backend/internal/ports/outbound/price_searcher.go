package outbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
)

// PriceSearcher declares all the methods the adapter must implement
type PriceSearcher interface {
	Search(ctx context.Context, criteria pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error)
}
