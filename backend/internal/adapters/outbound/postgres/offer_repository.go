package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	entoffer "github.com/LoneWolfPR/MedMarket/backend/ent/offer"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// OfferRepository is the struct that defines the properties for the adapter
type OfferRepository struct {
	client *ent.Client
	logger *slog.Logger
}

var _ outbound.OfferRepository = (*OfferRepository)(nil)

// NewOfferRepositoryParams holds the params necessary for constructing the instance
type NewOfferRepositoryParams struct {
	Client *ent.Client
	Logger *slog.Logger
}

// NewOfferRepository constructs the instance
func NewOfferRepository(p NewOfferRepositoryParams) (*OfferRepository, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &OfferRepository{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Create inserts a new offer record into the database
func (r *OfferRepository) Create(ctx context.Context, offer *pharmacy.Offer) (*pharmacy.Offer, error) {
	offerRecord, err := r.client.Offer.Create().
		SetPrescriptionID(offer.PrescriptionID).
		SetPriceCents(offer.Quote.Price().Cents()).
		SetPharmacyID(offer.Quote.PharmacyID()).
		SetPharmacyItemID(offer.Quote.PharmacyItemID()).
		SetExpiresAt(offer.ExpiresAt).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error saving offer record: %w", err)
	}
	r.logger.DebugContext(ctx, "offer record created", "id", offerRecord.ID)
	return mapToDomainOffer(offerRecord)
}

// GetByID fetches an offer record by its unique id
func (r *OfferRepository) GetByID(ctx context.Context, id uuid.UUID) (*pharmacy.Offer, error) {
	offerRecord, err := r.client.Offer.Query().
		Where(entoffer.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrOfferNotFound
		}
		return nil, fmt.Errorf("error fetching offer record: %w", err)
	}
	return mapToDomainOffer(offerRecord)
}

func mapToDomainOffer(offerRecord *ent.Offer) (*pharmacy.Offer, error) {
	price, err := shared.NewMoneyFromCents(offerRecord.PriceCents)
	if err != nil {
		return nil, fmt.Errorf("error with price in record: %w", err)
	}
	quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     offerRecord.PharmacyID,
		PharmacyItemID: offerRecord.PharmacyItemID,
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing quote from record: %w", err)
	}
	return &pharmacy.Offer{
		ID:             offerRecord.ID,
		Quote:          quote,
		ExpiresAt:      offerRecord.ExpiresAt,
		PrescriptionID: offerRecord.PrescriptionID,
	}, nil
}
