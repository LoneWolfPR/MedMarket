package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"

	"github.com/google/uuid"
)

// PrescriptionService implements the port to allow access to user
// prescription information
type PrescriptionService struct {
	logger           *slog.Logger
	prescriptionRepo outbound.PrescriptionRepository
	fileStorage      outbound.FileStorage
}

var _ inbound.PrescriptionService = (*PrescriptionService)(nil)

// NewPrescriptionServiceParams holds the input params necessary
// to construct a new instance of the service
type NewPrescriptionServiceParams struct {
	Logger           *slog.Logger
	PrescriptionRepo outbound.PrescriptionRepository
	FileStorage      outbound.FileStorage
}

// NewPrescriptionService takes the input a creates a new instance
// of the PrescriptionService
func NewPrescriptionService(p NewPrescriptionServiceParams) (*PrescriptionService, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.PrescriptionRepo == nil {
		return nil, errors.New("prescription repository is missing")
	}
	if p.FileStorage == nil {
		return nil, errors.New("file storage is missing")
	}
	return &PrescriptionService{
		logger:           p.Logger,
		prescriptionRepo: p.PrescriptionRepo,
		fileStorage:      p.FileStorage,
	}, nil
}

// Upload takes the user input, validates it, then stores the file in S3 storage and writes the
// information to the database
func (s *PrescriptionService) Upload(
	ctx context.Context,
	pi inbound.PrescriptionInput,
) (inbound.PrescriptionView, error) {
	validContentTypes := map[string]string{
		"application/pdf": "pdf",
		"image/png":       "png",
		"image/jpeg":      "jpg",
	}

	keyExt, ok := validContentTypes[pi.ContentType]
	if !ok {
		return inbound.PrescriptionView{}, inbound.ErrInvalidFile
	}
	docKey := fmt.Sprintf("prescriptions/%s/%s.%s", pi.UserID, uuid.New(), keyExt)

	medStrength, err := shared.NewMedStrength(
		pi.MedStrengthValue,
		pi.MedStrengthUnit,
	)
	if err != nil {
		return inbound.PrescriptionView{}, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}

	rx, err := prescription.NewPrescription(prescription.NewPrescriptionParams{
		UserID:        pi.UserID,
		DocumentKey:   docKey,
		PhysicianName: pi.PhysicianName,
		MedName:       pi.MedName,
		MedStrength:   medStrength,
		Qty:           pi.Qty,
	})
	if err != nil {
		return inbound.PrescriptionView{}, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}

	if err = s.fileStorage.Put(ctx, docKey, pi.ContentType, pi.File); err != nil {
		return inbound.PrescriptionView{}, fmt.Errorf("error writing file to storage: %w", err)
	}

	newPrescription, err := s.prescriptionRepo.Create(ctx, rx)
	if err != nil {
		return inbound.PrescriptionView{}, fmt.Errorf("error writing prescription record to database: %w", err)
	}

	docURL, err := s.fileStorage.GetPresignedURL(ctx, docKey)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"error generating presigned url",
			"prescription_id", newPrescription.ID,
			"error", err)
	}

	return inbound.PrescriptionView{
		Prescription: *newPrescription,
		DocumentURL:  docURL,
	}, nil
}

// GetPrescription takes a user id and prescription id and returns the matching record
func (s *PrescriptionService) GetPrescription(
	ctx context.Context,
	userID, id uuid.UUID,
) (inbound.PrescriptionView, error) {
	rx, err := s.prescriptionRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, outbound.ErrPrescriptionNotFound) {
			return inbound.PrescriptionView{},
				fmt.Errorf("%w: %w", inbound.ErrPrescriptionNotFound, err)
		}
		return inbound.PrescriptionView{}, fmt.Errorf("error getting prescription: %w", err)
	}
	if rx.UserID != userID {
		return inbound.PrescriptionView{}, inbound.ErrPrescriptionNotFound
	}
	docURL, err := s.fileStorage.GetPresignedURL(ctx, rx.DocumentKey)
	if err != nil {
		s.logger.ErrorContext(ctx, "error generating presigned url", "prescription_id", rx.ID, "error", err)
	}
	return inbound.PrescriptionView{
		Prescription: ptr.Deref(rx),
		DocumentURL:  docURL,
	}, nil
}

// GetPrescriptionList takes a user id and returns a list of all prescriptions for a user
func (s *PrescriptionService) GetPrescriptionList(
	ctx context.Context,
	userID uuid.UUID,
) ([]inbound.PrescriptionView, error) {
	result := []inbound.PrescriptionView{}
	rxList, err := s.prescriptionRepo.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching prescription list: %w", err)
	}
	for _, rx := range rxList {
		docURL, err := s.fileStorage.GetPresignedURL(ctx, rx.DocumentKey)
		// If there's an error generating a url we want to log it, but continue adding the information to the list
		// without failing
		if err != nil {
			s.logger.ErrorContext(ctx, "error generating presigned url", "prescription_id", rx.ID, "error", err)
		}
		result = append(result, inbound.PrescriptionView{
			Prescription: rx,
			DocumentURL:  docURL,
		})
	}
	return result, nil
}
