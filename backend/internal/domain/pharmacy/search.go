package pharmacy

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// SearchCriteria is a struct that holds all the information for searching a
// pharmacy for a particular med in a prescription
type SearchCriteria struct {
	medName     string
	medStrength shared.MedStrength
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingMedName     = errors.New("med name is missing")
	ErrMissingMedStrength = errors.New("med strength is missing")
)

// NewSearchCriteriaParams holds the input params for creating a search
type NewSearchCriteriaParams struct {
	MedName     string
	MedStrength shared.MedStrength
}

// NewSearchCriteria is the constructor for building criteria for searching a pharmacy
func NewSearchCriteria(p NewSearchCriteriaParams) (SearchCriteria, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(p.MedName))
	if normalizedName == "" {
		return SearchCriteria{}, ErrMissingMedName
	}

	if p.MedStrength.IsZero() {
		return SearchCriteria{}, ErrMissingMedStrength
	}

	return SearchCriteria{
		medName:     normalizedName,
		medStrength: p.MedStrength,
	}, nil
}

// MedName is a getter
func (s SearchCriteria) MedName() string {
	return s.medName
}

// MedStrength is a getter
func (s SearchCriteria) MedStrength() shared.MedStrength {
	return s.medStrength
}
