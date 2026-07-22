package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
)

// ShippingHandler is the adapter for handling shipping updates
type ShippingHandler struct {
	logger *slog.Logger
	svc    inbound.ShippingService
}

// NewShippingHandlerParams holds the parameters necessary to construct the handler
type NewShippingHandlerParams struct {
	Logger *slog.Logger
	Svc    inbound.ShippingService
}

// NewShippingHandler constructs an instance of the handler
func NewShippingHandler(p NewShippingHandlerParams) (*ShippingHandler, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Svc == nil {
		return nil, errors.New("service is missing")
	}
	return &ShippingHandler{
		logger: p.Logger,
		svc:    p.Svc,
	}, nil
}

// ShippingWebhook takes the input from a call to the webhook and attempts to update
// the order workflow
func (h *ShippingHandler) ShippingWebhook(
	ctx context.Context,
	req openapi.ShippingWebhookRequestObject,
) (openapi.ShippingWebhookResponseObject, error) {
	update := inbound.ShippingUpdate{
		Status:     req.Body.Status,
		TrackingID: req.Body.TrackingId,
		OccurredAt: req.Body.OccurredAt,
	}
	if err := h.svc.SendShippingUpdate(ctx, req.OrderID, update); err != nil {
		if errors.Is(err, inbound.ErrOrderWorkflowNotFound) {
			return openapi.ShippingWebhook404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse{
					Message: inbound.ErrOrderWorkflowNotFound.Error(),
				},
			}, nil
		}
		return nil, fmt.Errorf("error signaling order workflow: %w", err)
	}
	return openapi.ShippingWebhook202Response{}, nil
}
