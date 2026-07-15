package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// PriceSearchService is used to fetch quotes for medication
type PriceSearchService struct {
	logger    *slog.Logger
	rxRepo    outbound.PrescriptionRepository
	pharmRepo outbound.PharmacyRepository
	searcher  outbound.PriceSearcher
}

var _ inbound.PriceSearchService = (*PriceSearchService)(nil)

// NewPriceSearchServiceParams holds the params necessary to construct the service
type NewPriceSearchServiceParams struct {
	Logger    *slog.Logger
	RxRepo    outbound.PrescriptionRepository
	PharmRepo outbound.PharmacyRepository
	Searcher  outbound.PriceSearcher
}

// NewPriceSearchService constructs the service
func NewPriceSearchService(p NewPriceSearchServiceParams) (*PriceSearchService, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.RxRepo == nil {
		return nil, errors.New("prescription repo is missing")
	}
	if p.PharmRepo == nil {
		return nil, errors.New("pharmacy repo is missing")
	}
	if p.Searcher == nil {
		return nil, errors.New("price searcher is missing")
	}
	return &PriceSearchService{
		logger:    p.Logger,
		rxRepo:    p.RxRepo,
		pharmRepo: p.PharmRepo,
		searcher:  p.Searcher,
	}, nil
}

// GetQuoteList takes a prescription ID, pulls the record from the database, validates
// that the provided user id owns it, then searches pharmacies for a price quote on the
// medication.
func (s *PriceSearchService) GetQuoteList(
	ctx context.Context,
	userID, rxID uuid.UUID,
) ([]inbound.QuoteView, error) {
	rx, err := s.rxRepo.GetByID(ctx, rxID)
	if err != nil {
		if errors.Is(err, outbound.ErrPrescriptionNotFound) {
			return nil, fmt.Errorf("%w: %w", inbound.ErrPrescriptionNotFound, err)
		}
		return nil, fmt.Errorf("error getting presciption: %w", err)
	}
	if rx.UserID != userID {
		return nil, inbound.ErrPrescriptionNotFound
	}

	pharms, err := s.pharmRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing pharmacies: %w", err)
	}
	pharmList := map[uuid.UUID]string{}
	for _, pharm := range pharms {
		pharmList[pharm.ID] = pharm.Name
	}

	searchInput, err := pharmacy.NewSearchCriteria(pharmacy.NewSearchCriteriaParams{
		MedName:     rx.MedName,
		MedStrength: rx.MedStrength,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating search input: %w", err)
	}

	quoteResults, err := s.searcher.Search(ctx, searchInput)
	if err != nil {
		return nil, fmt.Errorf("error searching: %w", err)
	}
	results := []inbound.QuoteView{}
	for _, result := range quoteResults {
		currPharm, ok := pharmList[result.PharmacyID()]
		if !ok {
			s.logger.ErrorContext(ctx, "pharmacy not found for that id", "pharmacy id", result.PharmacyID())
			continue
		}
		totalCents := result.Price().Cents() * int64(rx.Qty)
		total, err := shared.NewMoneyFromCents(totalCents)
		if err != nil {
			s.logger.ErrorContext(ctx, "error with total cents", "total cents", totalCents, "error", err)
			continue
		}
		newQuoteView := inbound.QuoteView{
			Quote:        result,
			PharmacyName: currPharm,
			Total:        total,
		}
		results = append(results, newQuoteView)
	}
	return results, nil
}
