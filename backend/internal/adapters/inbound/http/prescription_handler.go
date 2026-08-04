package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
)

// PrescriptionHandler is an adapter for processing calls for user prescriptions
type PrescriptionHandler struct {
	logger *slog.Logger
	svc    inbound.PrescriptionService
}

// NewPrescriptionHandlerParams holds the parameters required to construct
// a new instance
type NewPrescriptionHandlerParams struct {
	Logger *slog.Logger
	Svc    inbound.PrescriptionService
}

const (
	formNamePhysician     = "physicianName"
	formNameMedName       = "medName"
	formNameStrengthValue = "strengthValue"
	formNameStrengthUnit  = "strengthUnit"
	formNameQuantity      = "quantity"
	formNameDocument      = "document"
	uploadLimit           = 10 << 20
)

// NewPrescriptionHandler builds a new instance of the adapter
func NewPrescriptionHandler(p NewPrescriptionHandlerParams) (*PrescriptionHandler, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &PrescriptionHandler{
		logger: p.Logger,
		svc:    p.Svc,
	}, nil
}

// UploadPrescription takes a request, validates it and sends the information for
// processing by the service
func (h *PrescriptionHandler) UploadPrescription(
	ctx context.Context,
	req openapi.UploadPrescriptionRequestObject,
) (openapi.UploadPrescriptionResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.UploadPrescription401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}
	var docData []byte
	uploadInput := inbound.PrescriptionInput{UserID: userID}

	// Loop through form parts and add parameters. We do this in case the parts
	// come in an unanticipated order so we don't lose document upload data
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break // no more parts
		}
		if err != nil {
			return openapi.UploadPrescription400JSONResponse{
				BadRequestJSONResponse: validationBadRequestBody(),
			}, nil
		}
		switch part.FormName() {
		case formNamePhysician:
			physician, err := io.ReadAll(part)
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			uploadInput.PhysicianName = string(physician)
		case formNameMedName:
			medName, err := io.ReadAll(part)
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			uploadInput.MedName = string(medName)
		case formNameStrengthValue:
			medStrengthValue, err := io.ReadAll(part)
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			uploadInput.MedStrengthValue = string(medStrengthValue)
		case formNameStrengthUnit:
			medStrengthUnit, err := io.ReadAll(part)
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			uploadInput.MedStrengthUnit = string(medStrengthUnit)
		case formNameQuantity:
			qtyBytes, err := io.ReadAll(part)
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			qty, err := strconv.Atoi(string(qtyBytes))
			if err != nil {
				return openapi.UploadPrescription400JSONResponse{
					BadRequestJSONResponse: validationBadRequestBody(),
				}, nil
			}
			uploadInput.Qty = qty
		case formNameDocument:
			docData, err = io.ReadAll(io.LimitReader(part, uploadLimit+1))
			if err != nil {
				return nil, err
			}
		}
	}
	// Check docData
	if len(docData) == 0 {
		return openapi.UploadPrescription400JSONResponse{
			BadRequestJSONResponse: openapi.BadRequestJSONResponse{
				Message: inbound.ErrInvalidFile.Error(),
			},
		}, nil
	}
	if len(docData) > uploadLimit {
		return openapi.UploadPrescription400JSONResponse{
			BadRequestJSONResponse: openapi.BadRequestJSONResponse{
				Message: msgFileTooLarge,
			},
		}, nil
	}
	uploadInput.ContentType = http.DetectContentType(docData)
	uploadInput.File = bytes.NewReader(docData)

	// Upload and save
	newRxView, err := h.svc.Upload(ctx, uploadInput)
	if err != nil {
		switch {
		case errors.Is(err, inbound.ErrInvalidFile):
			return openapi.UploadPrescription400JSONResponse{
				BadRequestJSONResponse: openapi.BadRequestJSONResponse{
					Message: inbound.ErrInvalidFile.Error(),
				},
			}, nil
		case errors.Is(err, inbound.ErrValidation):
			return openapi.UploadPrescription400JSONResponse{
				BadRequestJSONResponse: validationBadRequestBody(),
			}, nil
		default:
			return nil, err
		}
	}
	return openapi.UploadPrescription201JSONResponse(toPrescriptionResponse(newRxView)), nil
}

// ListPrescriptions handles a request for getting a users list of prescriptions
func (h *PrescriptionHandler) ListPrescriptions(
	ctx context.Context,
	_ openapi.ListPrescriptionsRequestObject,
) (openapi.ListPrescriptionsResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.ListPrescriptions401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}
	prescriptions, err := h.svc.GetPrescriptionList(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := []openapi.PrescriptionResponse{}
	for _, rx := range prescriptions {
		listItem := toPrescriptionResponse(rx)
		resp = append(resp, listItem)
	}
	return openapi.ListPrescriptions200JSONResponse(resp), nil
}

func toPrescriptionResponse(rxView inbound.PrescriptionView) openapi.PrescriptionResponse {
	return openapi.PrescriptionResponse{
		Id:            rxView.Prescription.ID,
		PhysicianName: rxView.Prescription.PhysicianName,
		MedName:       rxView.Prescription.MedName,
		StrengthValue: rxView.Prescription.MedStrength.Value(),
		StrengthUnit:  rxView.Prescription.MedStrength.Unit(),
		Quantity:      rxView.Prescription.Qty,
		DocumentUrl:   rxView.DocumentURL,
	}
}
