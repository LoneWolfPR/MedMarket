package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	entpharmacy "github.com/LoneWolfPR/MedMarket/backend/ent/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"

	"github.com/google/uuid"
)

// PharmacyRepository is the struct that defines the properties for the adapter
type PharmacyRepository struct {
	client *ent.Client
	logger *slog.Logger
}

var _ outbound.PharmacyRepository = (*PharmacyRepository)(nil)

// NewPharmacyRepositoryParams holds the necessary input params for the adapter
type NewPharmacyRepositoryParams struct {
	Client *ent.Client
	Logger *slog.Logger
}

// NewPharmacyRepository is the constructor to set up the adapter
func NewPharmacyRepository(p NewPharmacyRepositoryParams) (*PharmacyRepository, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &PharmacyRepository{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Create takes the supplied input and creates a new pharmacy record in the database
func (r *PharmacyRepository) Create(
	ctx context.Context,
	p *pharmacy.Pharmacy,
) (*pharmacy.Pharmacy, error) {
	pharmacyRecord, err := r.client.Pharmacy.Create().
		SetName(p.Name).
		SetAddressStreet1(p.Address.Street1).
		SetAddressStreet2(p.Address.Street2).
		SetAddressCity(p.Address.City).
		SetAddressState(p.Address.State).
		SetAddressZip(p.Address.Zip).
		SetContactPhone(p.ContactPhone.String()).
		SetNpi(p.NPINum.String()).
		SetDea(p.DEANum.String()).
		SetNcpdp(p.NCPDPNum.String()).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, outbound.ErrPharmacyExists
		}
		return nil, fmt.Errorf("error creating pharmacy: %w", err)
	}
	return mapToDomainPharmacy(pharmacyRecord)
}

// GetByID takes a uuid and searches the database for a matching pharmacy
func (r *PharmacyRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*pharmacy.Pharmacy, error) {
	pharmacyRecord, err := r.client.Pharmacy.Query().
		Where(entpharmacy.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrPharmacyNotFound
		}
		return nil, fmt.Errorf("error searching for pharmacy: %w", err)
	}
	return mapToDomainPharmacy(pharmacyRecord)
}

// List returns a list of all pharmacy records in the database
func (r *PharmacyRepository) List(ctx context.Context) ([]pharmacy.Pharmacy, error) {
	pharmacyRecords, err := r.client.Pharmacy.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting pharmacy list: %w", err)
	}
	pharmacies := make([]pharmacy.Pharmacy, 0, len(pharmacyRecords))
	for _, record := range pharmacyRecords {
		pharm, err := mapToDomainPharmacy(record)
		if err != nil {
			// We don't want to fail if individual records have problems, but it should
			// be logged for tracking
			r.logger.WarnContext(ctx, "error mapping pharmacy", "id", record.ID, "error", err)
		} else {
			pharmacies = append(pharmacies, ptr.Deref(pharm))
		}
	}
	return pharmacies, nil
}

func mapToDomainPharmacy(pharmacyRecord *ent.Pharmacy) (*pharmacy.Pharmacy, error) {
	contactPhone, err := shared.NewPhone(pharmacyRecord.ContactPhone)
	if err != nil {
		return nil, fmt.Errorf("error with the phone on file: %w", err)
	}
	npiNum, err := pharmacy.NewNPI(pharmacyRecord.Npi)
	if err != nil {
		return nil, fmt.Errorf("error with the npi on file: %w", err)
	}
	deaNum, err := pharmacy.NewDEA(pharmacyRecord.Dea)
	if err != nil {
		return nil, fmt.Errorf("error with the dea number on file: %w", err)
	}
	ncpdpNum, err := pharmacy.NewNCPDP(pharmacyRecord.Ncpdp)
	if err != nil {
		return nil, fmt.Errorf("error with the ncpdp number on file: %w", err)
	}

	return &pharmacy.Pharmacy{
		ID:           pharmacyRecord.ID,
		Name:         pharmacyRecord.Name,
		ContactPhone: contactPhone,
		NPINum:       npiNum,
		DEANum:       deaNum,
		NCPDPNum:     ncpdpNum,
		Address: shared.Address{
			Street1: pharmacyRecord.AddressStreet1,
			Street2: pharmacyRecord.AddressStreet2,
			City:    pharmacyRecord.AddressCity,
			State:   pharmacyRecord.AddressState,
			Zip:     pharmacyRecord.AddressZip,
		},
	}, nil
}
