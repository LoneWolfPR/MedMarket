package pricesearch

import (
	"context"
	"fmt"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	internalshared "github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

// Activities holds the properties necessary for execution of methods
type Activities struct {
	repo    outbound.PharmacyRepository
	clients map[string]outbound.PharmacyClient
}

// NewActivitiesParams holds the necessary params to construct an instance
type NewActivitiesParams struct {
	Repo    outbound.PharmacyRepository
	Clients map[string]outbound.PharmacyClient
}

// NewActivities constructs a new instance
func NewActivities(p NewActivitiesParams) *Activities {
	return &Activities{
		repo:    p.Repo,
		clients: p.Clients,
	}
}

// PharmacyInfo contains the relevant info for each pharmacy
type PharmacyInfo struct {
	ID   uuid.UUID
	Code string
	Name string
}

// ListPharmaciesActivity uses the pharmacy repo to get a list of accessible pharmacies
func (a *Activities) ListPharmaciesActivity(ctx context.Context) ([]PharmacyInfo, error) {
	pharmacies, err := a.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching pharmacy list: %w", err)
	}
	pharmInfos := []PharmacyInfo{}
	for _, pharm := range pharmacies {
		if pharm.Code == "" || a.clients[pharm.Code] == nil {
			continue
		}
		newPharm := PharmacyInfo{
			ID:   pharm.ID,
			Code: pharm.Code,
			Name: pharm.Name,
		}
		pharmInfos = append(pharmInfos, newPharm)
	}
	return pharmInfos, nil
}

// SearchPharmacyInputParams holds the necessary search parameters
type SearchPharmacyInputParams struct {
	PharmInfo PharmacyInfo
	Input     SearchInput
}

// SearchPharmacyActivity takes a pharmacy to search along with the criteria
// in order to build a list of quotes for medication
func (a *Activities) SearchPharmacyActivity(
	ctx context.Context,
	p SearchPharmacyInputParams,
) ([]QuoteResult, error) {
	const invalidSearchErrLabel = "InvalidSearchInput"
	client := a.clients[p.PharmInfo.Code]
	if client == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("client with code %s does not exist", p.PharmInfo.Code),
			"MissingPharmacyClient",
			nil,
		)
	}
	medStrength, err := internalshared.NewMedStrength(p.Input.MedStrengthValue, p.Input.MedStrengthUnit)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(
			"invalid med strength in search",
			invalidSearchErrLabel,
			err,
		)
	}
	criteria, err := pharmacy.NewSearchCriteria(pharmacy.NewSearchCriteriaParams{
		MedName:     p.Input.MedName,
		MedStrength: medStrength,
	})
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(
			"invalid search criteria",
			invalidSearchErrLabel,
			err,
		)
	}
	results := []QuoteResult{}
	quotes, err := client.Search(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("error searching pharmacy %s: %w", p.PharmInfo.Name, err)
	}
	for _, quote := range quotes {
		results = append(results, QuoteResult{
			UnitPriceCents: quote.Price().Cents(),
			PharmacyID:     quote.PharmacyID(),
			PharmacyItemID: quote.PharmacyItemID(),
		})
	}
	return results, nil
}
