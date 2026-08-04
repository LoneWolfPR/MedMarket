package inbound

import (
	"context"
	"errors"
	"io"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var (
	ErrInvalidFile          = errors.New("invalid file")
	ErrPrescriptionNotFound = errors.New("prescription not found")
)

// PrescriptionInput is the struct that holds the properties
// necessary to store a user's prescription information
type PrescriptionInput struct {
	UserID           uuid.UUID
	PhysicianName    string
	MedName          string
	MedStrengthValue string
	MedStrengthUnit  string
	Qty              int
	File             io.Reader
	ContentType      string
}

// PrescriptionView contains both the prescription info and a presigned
// url for the prescription document
type PrescriptionView struct {
	Prescription prescription.Prescription
	DocumentURL  string
}

// PrescriptionService defines the methods that the service must
// implement for storing user prescription information
type PrescriptionService interface {
	Upload(ctx context.Context, pi PrescriptionInput) (PrescriptionView, error)
	GetPrescription(ctx context.Context, userID, id uuid.UUID) (PrescriptionView, error)
	GetPrescriptionList(ctx context.Context, userID uuid.UUID) ([]PrescriptionView, error)
}
