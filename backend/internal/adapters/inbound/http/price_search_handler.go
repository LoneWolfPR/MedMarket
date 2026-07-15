package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
)

// PriceSearchHandler is the adapter for handling incoming price search requests
type PriceSearchHandler struct {
	logger *slog.Logger
	svc    inbound.PriceSearchService
}

// NewPriceSearchHandlerParams holds the params necessary to construct an instance
type NewPriceSearchHandlerParams struct {
	Logger *slog.Logger
	Svc    inbound.PriceSearchService
}

// NewPriceSearchHandler constructs the instance
func NewPriceSearchHandler(p NewPriceSearchHandlerParams) (*PriceSearchHandler, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Svc == nil {
		return nil, errors.New("service is missing")
	}
	return &PriceSearchHandler{
		logger: p.Logger,
		svc:    p.Svc,
	}, nil
}

// SearchPrescriptionPrices processes the request to get a list of prices for the medication
// attached to a user's prescription
func (h *PriceSearchHandler) SearchPrescriptionPrices(
	ctx context.Context,
	req openapi.SearchPrescriptionPricesRequestObject,
) (openapi.SearchPrescriptionPricesResponseObject, error) {
	userID, ok := getUserIDKeyValue(ctx)
	if !ok {
		return openapi.SearchPrescriptionPrices401JSONResponse{
			UnauthorizedJSONResponse: unauthorizedBody(),
		}, nil
	}
	quotes, err := h.svc.GetQuoteList(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, inbound.ErrPrescriptionNotFound) {
			return openapi.SearchPrescriptionPrices404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse{
					Message: inbound.ErrPrescriptionNotFound.Error(),
				},
			}, nil
		}
		return nil, err
	}
	resp := []openapi.QuoteResponse{}
	for _, quote := range quotes {
		resp = append(resp, toQuoteResponse(quote))
	}
	return openapi.SearchPrescriptionPrices200JSONResponse(resp), nil
}

func toQuoteResponse(quote inbound.QuoteView) openapi.QuoteResponse {
	return openapi.QuoteResponse{
		PharmacyId:     quote.Quote.PharmacyID(),
		PharmacyItemId: quote.Quote.PharmacyItemID(),
		PharmacyName:   quote.PharmacyName,
		UnitPriceCents: quote.Quote.Price().Cents(),
		TotalCents:     quote.Total.Cents(),
	}
}
