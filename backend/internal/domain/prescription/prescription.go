package prescription

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// Prescription holds all the business information
// about a user's prescription for a particular medication
type Prescription struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DocumentKey   string
	PhysicianName string
	MedName       string
	MedStrength   shared.MedStrength
	Qty           int
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingUserID      = errors.New("user id is missing")
	ErrMissingDocKey      = errors.New("document key is missing")
	ErrMissingPhysName    = errors.New("physician name is missing")
	ErrMissingMedName     = errors.New("medication name is missing")
	ErrMissingMedStrength = errors.New("medication strength is missing")
	ErrInvalidQty         = errors.New("quantity is invalid")
)

// NewPrescriptionParams holds the input parameters for creating
// a new prescription entity
type NewPrescriptionParams struct {
	UserID        uuid.UUID
	DocumentKey   string
	PhysicianName string
	MedName       string
	MedStrength   shared.MedStrength
	Qty           int
}

// NewPrescription is the constructor to set up a new prescription entity
func NewPrescription(p NewPrescriptionParams) (*Prescription, error) {
	if p.UserID == uuid.Nil {
		return nil, ErrMissingUserID
	}
	if p.DocumentKey == "" {
		return nil, ErrMissingDocKey
	}
	trimmedPhysName := strings.TrimSpace(p.PhysicianName)
	if trimmedPhysName == "" {
		return nil, ErrMissingPhysName
	}
	trimmedMedName := strings.TrimSpace(p.MedName)
	if trimmedMedName == "" {
		return nil, ErrMissingMedName
	}
	if p.MedStrength.IsZero() {
		return nil, ErrMissingMedStrength
	}
	if p.Qty <= 0 {
		return nil, ErrInvalidQty
	}

	return &Prescription{
		UserID:        p.UserID,
		DocumentKey:   p.DocumentKey,
		PhysicianName: trimmedPhysName,
		MedName:       trimmedMedName,
		MedStrength:   p.MedStrength,
		Qty:           p.Qty,
	}, nil
}
