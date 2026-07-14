package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// Stable business codes for the two mock pharmacies. These are the binding key
// between a pharmacy row and its adapter: the seed writes them and the
// composition roots read them back via GetByCode to resolve each adapter's PK.
const (
	PharmacyACode = "mock-pharmacy-a"
	PharmacyBCode = "mock-pharmacy-b"
)

type pharmacySeed struct {
	code    string
	name    string
	npi     string
	dea     string
	ncpdp   string
	phone   string
	address shared.Address
}

// pharmacySeeds is the fixed set of pharmacies the app depends on to function.
// Regulatory IDs carry real checksums (NPI Luhn, DEA check digit); NewPharmacy
// re-validates them at seed time, so a bad value here fails startup loudly.
var pharmacySeeds = []pharmacySeed{
	{
		code:  PharmacyACode,
		name:  "Mock Pharmacy A",
		npi:   "1234567893",
		dea:   "AF1234563",
		ncpdp: "1234567",
		phone: "5551234567",
		address: shared.Address{
			Street1: "100 Market St",
			City:    "Springfield",
			State:   "IL",
			Zip:     "62704",
		},
	},
	{
		code:  PharmacyBCode,
		name:  "Mock Pharmacy B",
		npi:   "1245319599",
		dea:   "BX7654329",
		ncpdp: "7654321",
		phone: "5559876543",
		address: shared.Address{
			Street1: "200 Commerce Ave",
			City:    "Madison",
			State:   "WI",
			Zip:     "53703",
		},
	},
}

// SeedPharmacies idempotently ensures every required pharmacy row exists. It is
// safe to call from every binary at startup: existing rows are skipped, missing
// ones are created, and any other repo error fails loudly.
func SeedPharmacies(ctx context.Context, repo outbound.PharmacyRepository) error {
	for _, seed := range pharmacySeeds {
		_, err := repo.GetByCode(ctx, seed.code)
		switch {
		case err == nil:
			// Pharmacy already exists. Move on.
			continue
		case errors.Is(err, outbound.ErrPharmacyNotFound):
			// Pharmacy can be created
			phone, err := shared.NewPhone(seed.phone)
			if err != nil {
				return fmt.Errorf("error with phone on pharmacy seed %q: %w", seed.code, err)
			}
			npiNum, err := pharmacy.NewNPI(seed.npi)
			if err != nil {
				return fmt.Errorf("error with npi on pharmacy seed %q: %w", seed.code, err)
			}
			deaNum, err := pharmacy.NewDEA(seed.dea)
			if err != nil {
				return fmt.Errorf("error with dea on pharmacy seed %q: %w", seed.code, err)
			}
			ncpdpNum, err := pharmacy.NewNCPDP(seed.ncpdp)
			if err != nil {
				return fmt.Errorf("error with ncpdp on pharmacy seed %q: %w", seed.code, err)
			}
			pharm, err := pharmacy.NewPharmacy(pharmacy.NewPharmacyParams{
				Code:         seed.code,
				Name:         seed.name,
				ContactPhone: phone,
				NPINum:       npiNum,
				DEANum:       deaNum,
				NCPDPNum:     ncpdpNum,
				Address:      seed.address,
			})

			if err != nil {
				return fmt.Errorf("error creating pharmacy entity for seed %q: %w", seed.code, err)
			}

			_, err = repo.Create(ctx, pharm)
			if err != nil {
				return fmt.Errorf("error creating pharmacy with code %q: %w", seed.code, err)
			}
		default:
			// other errors are real db errors
			return fmt.Errorf("error checking db with pharmacy code %q: %w", seed.code, err)
		}
	}
	return nil
}
