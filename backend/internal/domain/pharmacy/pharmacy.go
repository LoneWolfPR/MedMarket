package pharmacy

import (
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// Pharmacy holds all the business information about a
// pharmacy entity
type Pharmacy struct {
	ID           uuid.UUID
	Name         string
	Address      shared.Address
	ContactPhone shared.Phone
	NPINum       NPI
	DEANum       DEA
	NCPDPNum     NCPDP
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingName    = errors.New("name is missing")
	ErrInvalidAddress = errors.New("address is invalid")
	ErrMissingPhone   = errors.New("contact phone is missing")
	ErrMissingNPI     = errors.New("NPI is missing")
	ErrMissingDEA     = errors.New("DEA number is missing")
	ErrMissingNCPDP   = errors.New("NCPDP number is missing")
)

// NewPharmacyParams holds all the necessary input parameters
// for creating anew pharmacy
type NewPharmacyParams struct {
	Name         string
	Address      shared.Address
	ContactPhone shared.Phone
	NPINum       NPI
	DEANum       DEA
	NCPDPNum     NCPDP
}

// NewPharmacy is the contructor for a new pharmcy entity
func NewPharmacy(p NewPharmacyParams) (*Pharmacy, error) {
	if p.Name == "" {
		return nil, ErrMissingName
	}
	if !p.Address.IsValid() {
		return nil, ErrInvalidAddress
	}
	if p.ContactPhone.IsZero() {
		return nil, ErrMissingPhone
	}
	if p.NPINum.IsZero() {
		return nil, ErrMissingNPI
	}
	if p.DEANum.IsZero() {
		return nil, ErrMissingDEA
	}
	if p.NCPDPNum.IsZero() {
		return nil, ErrMissingNCPDP
	}
	return &Pharmacy{
		Name:         p.Name,
		Address:      p.Address,
		ContactPhone: p.ContactPhone,
		NPINum:       p.NPINum,
		DEANum:       p.DEANum,
		NCPDPNum:     p.NCPDPNum,
	}, nil
}
