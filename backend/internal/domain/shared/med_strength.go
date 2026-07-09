package shared

import (
	"errors"
	"slices"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/numhelpers"
)

// MedStrength is a value type that holds the pieces of
// information necessary for a complete medication strength
type MedStrength struct {
	value string
	unit  string
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrInvalidMedStrengthValue = errors.New("invalid med strength value")
	ErrInvalidMedStrengthUnit  = errors.New("invalid med strength unit")
)

var acceptedUnits = []string{"mg", "ml", "mcg"}

// NewMedStrength is the constructor to populate the properties of
// a MedStrength value type object
func NewMedStrength(value, unit string) (MedStrength, error) {
	normalizedValue, err := numhelpers.NormalizeNumberString(value)
	if err != nil {
		return MedStrength{}, ErrInvalidMedStrengthValue
	}

	if normalizedValue == "0" || strings.HasPrefix(normalizedValue, "-") {
		return MedStrength{}, ErrInvalidMedStrengthValue
	}

	normalizedUnit := strings.ToLower(strings.TrimSpace(unit))
	if normalizedUnit == "" || !slices.Contains(acceptedUnits, normalizedUnit) {
		return MedStrength{}, ErrInvalidMedStrengthUnit
	}

	return MedStrength{
		value: normalizedValue,
		unit:  normalizedUnit,
	}, nil
}

// IsZero returns if a MedStrength type is empty
func (ms MedStrength) IsZero() bool {
	return ms == MedStrength{}
}

func (ms MedStrength) String() string {
	return ms.value + ms.unit
}
