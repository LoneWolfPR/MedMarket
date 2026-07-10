package pharmacy_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// validPharmacyParams returns a fully valid set of params; individual tests
// mutate a single field to exercise one invariant at a time.
func validPharmacyParams(t *testing.T) pharmacy.NewPharmacyParams {
	t.Helper()

	npi, err := pharmacy.NewNPI("1234567893")
	require.NoError(t, err)
	dea, err := pharmacy.NewDEA("AF1234563")
	require.NoError(t, err)
	ncpdp, err := pharmacy.NewNCPDP("1234567")
	require.NoError(t, err)
	phone, err := shared.NewPhone("5551234567")
	require.NoError(t, err)

	return pharmacy.NewPharmacyParams{
		Code: "test-pharmacy",
		Name: "Test Pharmacy",
		Address: shared.Address{
			Street1: "123 Main St",
			City:    "Springfield",
			State:   "IL",
			Zip:     "62704",
		},
		ContactPhone: phone,
		NPINum:       npi,
		DEANum:       dea,
		NCPDPNum:     ncpdp,
	}
}

func TestNewPharmacy_Valid(t *testing.T) {
	p := validPharmacyParams(t)

	got, err := pharmacy.NewPharmacy(p)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.Code, got.Code)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.Address, got.Address)
	assert.Equal(t, p.ContactPhone, got.ContactPhone)
	assert.Equal(t, p.NPINum, got.NPINum)
	assert.Equal(t, p.DEANum, got.DEANum)
	assert.Equal(t, p.NCPDPNum, got.NCPDPNum)
	assert.Equal(t, uuid.Nil, got.ID, "id is assigned by the persistence layer, not the constructor")
}

func TestNewPharmacy_Invalid(t *testing.T) {
	tests := map[string]struct {
		mutate  func(p *pharmacy.NewPharmacyParams)
		wantErr error
	}{
		"missing code": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.Code = "" },
			wantErr: pharmacy.ErrMissingCode,
		},
		"missing name": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.Name = "" },
			wantErr: pharmacy.ErrMissingName,
		},
		"invalid address": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.Address = shared.Address{} },
			wantErr: pharmacy.ErrInvalidAddress,
		},
		"missing phone": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.ContactPhone = shared.Phone{} },
			wantErr: pharmacy.ErrMissingPhone,
		},
		"missing npi": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.NPINum = pharmacy.NPI{} },
			wantErr: pharmacy.ErrMissingNPI,
		},
		"missing dea": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.DEANum = pharmacy.DEA{} },
			wantErr: pharmacy.ErrMissingDEA,
		},
		// Expects ErrMissingNCPDP for consistency with the other five guards.
		// NewPharmacy currently returns ErrInvalidNCPDP here (line ~62) — this
		// case fails until that is corrected. See note to reviewer.
		"missing ncpdp": {
			mutate:  func(p *pharmacy.NewPharmacyParams) { p.NCPDPNum = pharmacy.NCPDP{} },
			wantErr: pharmacy.ErrMissingNCPDP,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := validPharmacyParams(t)
			tc.mutate(&p)

			got, err := pharmacy.NewPharmacy(p)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, got, "no pharmacy returned on error")
		})
	}
}
