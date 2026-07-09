package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var (
	ErrPharmacyExists   = errors.New("pharmacy already exists")
	ErrPharmacyNotFound = errors.New("pharmacy was not found")
)

// PharmacyRepository declares the methods the adapters have to implement
type PharmacyRepository interface {
	Create(ctx context.Context, p *pharmacy.Pharmacy) (*pharmacy.Pharmacy, error)
	GetByID(ctx context.Context, id uuid.UUID) (*pharmacy.Pharmacy, error)
	List(ctx context.Context) ([]pharmacy.Pharmacy, error)
}
