package postgres

import (
	"context"
	"errors"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// PrescriptionRepository is the struct that defines the properties
// for the adapter
type PrescriptionRepository struct {
	client *ent.Client
	logger *slog.Logger
}

var _ outbound.PrescriptionRepository = (*PrescriptionRepository)(nil)

// NewPrescriptionRepositoryParams contains the params necessary to
// construct an instance
type NewPrescriptionRepositoryParams struct {
	Client *ent.Client
	Logger *slog.Logger
}

// NewPrescriptionRepository is the constructor for building a new instance
// of the adapter
func NewPrescriptionRepository(p NewPrescriptionRepositoryParams) (*PrescriptionRepository, error) {
	if p.Client == nil {
		return nil, errors.New("ent client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &PrescriptionRepository{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Create inserts a new prescription record
func (r *PrescriptionRepository) Create(
	ctx context.Context,
	p *prescription.Prescription,
) (*prescription.Prescription, error) {
	return nil, nil
}

// GetByID takes a prescription ID and fetches the record
func (r *PrescriptionRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*prescription.Prescription, error) {
	return nil, nil
}

// List fetches all prescription records for an authenticated user
func (r *PrescriptionRepository) List(
	ctx context.Context,
	id uuid.UUID,
) ([]prescription.Prescription, error) {
	rxList := []prescription.Prescription{}
	return rxList, nil
}
