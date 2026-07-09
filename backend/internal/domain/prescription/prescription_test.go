package prescription_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// validPrescriptionParams returns a fully valid set of params; individual tests
// mutate a single field to exercise one invariant at a time.
func validPrescriptionParams(t *testing.T) prescription.NewPrescriptionParams {
	t.Helper()

	strength, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)

	return prescription.NewPrescriptionParams{
		UserID:        uuid.New(),
		DocumentKey:   "prescriptions/abc123.pdf",
		PhysicianName: "Dr. Smith",
		MedName:       "Lipitor",
		MedStrength:   strength,
		Qty:           30,
	}
}

func TestNewPrescription_Valid(t *testing.T) {
	p := validPrescriptionParams(t)

	got, err := prescription.NewPrescription(p)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.UserID, got.UserID)
	assert.Equal(t, p.DocumentKey, got.DocumentKey)
	assert.Equal(t, p.PhysicianName, got.PhysicianName)
	assert.Equal(t, p.MedName, got.MedName)
	assert.Equal(t, p.MedStrength, got.MedStrength)
	assert.Equal(t, p.Qty, got.Qty)
	assert.Equal(t, uuid.Nil, got.ID, "id is assigned by the persistence layer, not the constructor")
}

// Both name fields are stored trimmed; casing is left untouched (SearchCriteria
// owns lowercase normalization).
func TestNewPrescription_TrimsNames(t *testing.T) {
	p := validPrescriptionParams(t)
	p.PhysicianName = "  Dr. Smith  "
	p.MedName = "  Lipitor  "

	got, err := prescription.NewPrescription(p)

	require.NoError(t, err)
	assert.Equal(t, "Dr. Smith", got.PhysicianName)
	assert.Equal(t, "Lipitor", got.MedName)
}

func TestNewPrescription_Invalid(t *testing.T) {
	tests := map[string]struct {
		mutate  func(p *prescription.NewPrescriptionParams)
		wantErr error
	}{
		"missing user id": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.UserID = uuid.Nil },
			wantErr: prescription.ErrMissingUserID,
		},
		"missing document key": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.DocumentKey = "" },
			wantErr: prescription.ErrMissingDocKey,
		},
		"blank physician name": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.PhysicianName = "   " },
			wantErr: prescription.ErrMissingPhysName,
		},
		"blank med name": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.MedName = "   " },
			wantErr: prescription.ErrMissingMedName,
		},
		"missing med strength": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.MedStrength = shared.MedStrength{} },
			wantErr: prescription.ErrMissingMedStrength,
		},
		"zero quantity": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.Qty = 0 },
			wantErr: prescription.ErrInvalidQty,
		},
		"negative quantity": {
			mutate:  func(p *prescription.NewPrescriptionParams) { p.Qty = -1 },
			wantErr: prescription.ErrInvalidQty,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := validPrescriptionParams(t)
			tc.mutate(&p)

			got, err := prescription.NewPrescription(p)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, got, "no prescription returned on error")
		})
	}
}
