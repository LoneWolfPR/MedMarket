package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	entprescription "github.com/LoneWolfPR/MedMarket/backend/ent/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"

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
	rxRecord, err := r.client.Prescription.Create().
		SetUserID(p.UserID).
		SetDocumentKey(p.DocumentKey).
		SetPhysicianName(p.PhysicianName).
		SetMedName(p.MedName).
		SetMedStrengthValue(p.MedStrength.Value()).
		SetMedStrengthUnit(p.MedStrength.Unit()).
		SetQuantity(p.Qty).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating prescription record: %w", err)
	}
	r.logger.DebugContext(ctx, "prescription record created", "prescription_id", rxRecord.ID)
	return mapToDomainPrescription(rxRecord)
}

// GetByID takes a prescription ID and fetches the record
func (r *PrescriptionRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*prescription.Prescription, error) {
	rxRecord, err := r.client.Prescription.Query().
		Where(entprescription.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrPrescriptionNotFound
		}
		return nil, fmt.Errorf("error fetching prescription by id: %w", err)
	}
	r.logger.DebugContext(ctx, "prescription record found", "prescription_id", id)
	return mapToDomainPrescription(rxRecord)
}

// List fetches all prescription records for an authenticated user
func (r *PrescriptionRepository) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]prescription.Prescription, error) {
	rxList := []prescription.Prescription{}
	rxRecordList, err := r.client.Prescription.Query().
		Where(entprescription.UserIDEQ(userID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching prescriptions for user: %w", err)
	}
	for _, rxRecord := range rxRecordList {
		rx, err := mapToDomainPrescription(rxRecord)
		if err != nil {
			r.logger.ErrorContext(
				ctx,
				"error mapping prescription record",
				"user_id", userID,
				"prescription_id", rxRecord.ID,
				"error", err)
		} else {
			rxList = append(rxList, ptr.Deref(rx))
		}
	}
	return rxList, nil
}

func mapToDomainPrescription(rxRecord *ent.Prescription) (*prescription.Prescription, error) {
	medStrength, err := shared.NewMedStrength(
		rxRecord.MedStrengthValue,
		rxRecord.MedStrengthUnit,
	)
	if err != nil {
		return nil, fmt.Errorf("error mapping prescription record: %w", err)
	}
	rx := &prescription.Prescription{
		ID:            rxRecord.ID,
		UserID:        rxRecord.UserID,
		DocumentKey:   rxRecord.DocumentKey,
		PhysicianName: rxRecord.PhysicianName,
		MedName:       rxRecord.MedName,
		MedStrength:   medStrength,
		Qty:           rxRecord.Quantity,
	}
	return rx, nil
}
