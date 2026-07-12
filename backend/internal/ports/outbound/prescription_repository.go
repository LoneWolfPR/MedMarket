package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var ErrPrescriptionNotFound = errors.New("prescription not found")

// PrescriptionRepository declares the methods the adapters must implement
type PrescriptionRepository interface {
	Create(ctx context.Context, p *prescription.Prescription) (*prescription.Prescription, error)
	GetByID(ctx context.Context, id uuid.UUID) (*prescription.Prescription, error)
	List(ctx context.Context, id uuid.UUID) ([]prescription.Prescription, error)
}
