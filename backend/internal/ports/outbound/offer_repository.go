package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var ErrOfferNotFound = errors.New("offer not found")

// OfferRepository defines the methods the adapters must implement
type OfferRepository interface {
	Create(ctx context.Context, offer *pharmacy.Offer) (*pharmacy.Offer, error)
	GetByID(ctx context.Context, id uuid.UUID) (*pharmacy.Offer, error)
}
