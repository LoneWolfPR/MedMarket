package pharmacya

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/httpx"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// PharmacyA is the adapter for making queries against Pharmacy A
type PharmacyA struct {
	client  *http.Client
	logger  *slog.Logger
	id      uuid.UUID
	secret  string
	baseURL string
}

var _ outbound.PharmacyClient = (*PharmacyA)(nil)

// NewPharmacyAParams are the input params necessary for the adapter
type NewPharmacyAParams struct {
	Client  *http.Client
	Logger  *slog.Logger
	ID      uuid.UUID
	Secret  string
	BaseURL string
}

// NewPharmacyA constructs a new instance of the PharmacyA adapter
func NewPharmacyA(p NewPharmacyAParams) (*PharmacyA, error) {
	if p.Client == nil {
		return nil, errors.New("http client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.ID == uuid.Nil {
		return nil, errors.New("pharmacy id is missing")
	}
	if p.Secret == "" {
		return nil, errors.New("api secret is missing")
	}
	if p.BaseURL == "" {
		return nil, errors.New("api base url is missing")
	}
	return &PharmacyA{
		client:  p.Client,
		logger:  p.Logger,
		secret:  p.Secret,
		baseURL: p.BaseURL,
		id:      p.ID,
	}, nil
}

// Search related structs
type pharmACriteria struct {
	MedName     string `json:"drug"`
	MedStrength string `json:"strength"`
}

type searchResultItem struct {
	SKU        string `json:"sku"`
	PriceCents int64  `json:"priceCents"`
}

type searchAPIResponse struct {
	Results []searchResultItem `json:"results"`
}

// Search queries the api for a list of prices for a specific medication by name and strength
func (pa *PharmacyA) Search(
	ctx context.Context,
	c pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	searchPath := pa.baseURL + "/api/v1/search"
	priceQuoteList := []pharmacy.PriceQuote{}

	criteria := pharmACriteria{
		MedName:     c.MedName(),
		MedStrength: c.MedStrength().String(),
	}

	jsonData, err := json.Marshal(criteria)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error constructing request: %w", err)
	}
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("X-Api-Key", pa.secret)

	resp, err := pa.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error searching: %w", err)
	}

	//nolint:errcheck // unnecessary
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pharmacy A search failed: status %d: %s", resp.StatusCode, bodyBytes)
	}

	var apiResp searchAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	for _, result := range apiResp.Results {
		trimmedSKU := strings.TrimSpace(result.SKU)
		if trimmedSKU == "" {
			pa.logger.WarnContext(ctx, "empty sku on item")
			continue
		}

		priceCents, err := shared.NewMoneyFromCents(result.PriceCents)
		if err != nil {
			pa.logger.WarnContext(ctx, "error reading price on item", "sku", trimmedSKU, "error", err)
			continue
		}

		quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
			Price:          priceCents,
			PharmacyID:     pa.id,
			PharmacyItemID: trimmedSKU,
		})
		if err != nil {
			pa.logger.WarnContext(ctx, "error creating quote", "error", err)
			continue
		}
		priceQuoteList = append(priceQuoteList, quote)
	}

	return priceQuoteList, nil
}

// Order related structs
type orderBody struct {
	SKU      string       `json:"sku"`
	Qty      int          `json:"quantity"`
	Shipping shippingAddr `json:"shipping"`
}

type shippingAddr struct {
	Name    string `json:"name"`
	Street1 string `json:"street1"`
	Street2 string `json:"street2"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
}

type orderAPIResponse struct {
	OrderID    string `json:"orderId"`
	TrackingID string `json:"trackingId"`
	Status     string `json:"status"`
	TotalCents int    `json:"totalCents"`
}

// PlaceOrder sends an order request to the api
func (pa *PharmacyA) PlaceOrder(
	ctx context.Context,
	i pharmacy.OrderInput,
) (pharmacy.OrderResult, error) {
	orderPath := pa.baseURL + "/api/v1/order"
	addr := i.ShippingAddress()
	orderInput := orderBody{
		SKU: i.PharmacyItemID(),
		Qty: i.Qty(),
		Shipping: shippingAddr{
			Name:    i.RecipientName(),
			Street1: addr.Street1,
			Street2: addr.Street2,
			City:    addr.City,
			State:   addr.State,
			Zip:     addr.Zip,
		},
	}

	jsonData, err := json.Marshal(orderInput)
	if err != nil {
		newErr := outbound.NewPlaceOrderError(
			outbound.KindRejected,
			fmt.Errorf("error marshaling json data: %w", err),
		)
		return pharmacy.OrderResult{}, newErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, orderPath, bytes.NewBuffer(jsonData))
	if err != nil {
		newErr := outbound.NewPlaceOrderError(
			outbound.KindRejected,
			fmt.Errorf("error constructing request: %w", err),
		)
		return pharmacy.OrderResult{}, newErr
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", i.IdempotencyKey())
	req.Header.Set("X-Api-Key", pa.secret)

	resp, err := pa.client.Do(req)
	if err != nil {
		return pharmacy.OrderResult{}, fmt.Errorf("error placing order: %w", err)
	}

	//nolint:errcheck // unnecessary
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		respError := fmt.Errorf(
			"pharmacy A failed to place order with status %d: %s",
			resp.StatusCode, bodyBytes)
		if !httpx.IsRetryableStatus(resp.StatusCode) {
			return pharmacy.OrderResult{}, outbound.NewPlaceOrderError(
				outbound.KindRejected,
				respError,
			)
		}
		return pharmacy.OrderResult{}, respError
	}

	var orderResp orderAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return pharmacy.OrderResult{}, fmt.Errorf("error decoding response: %w", err)
	}

	newTotal, err := shared.NewMoneyFromCents(int64(orderResp.TotalCents))
	if err != nil {
		// Logging but not failing if there's an issue on total since this is not
		// the price actually used. The price charged came from the offer at purchase
		// time
		pa.logger.ErrorContext(
			ctx,
			"error with total returned from order",
			"total", orderResp.TotalCents,
			"order id", orderResp.OrderID,
		)
	}
	result, err := pharmacy.NewOrderResult(pharmacy.NewOrderResultParams{
		PharmacyOrderID: orderResp.OrderID,
		TrackingID:      orderResp.TrackingID,
		NewTotal:        newTotal,
		OrderStatus:     orderResp.Status,
	})
	if err != nil {
		return pharmacy.OrderResult{}, fmt.Errorf("error parsing order response: %w", err)
	}
	return result, nil
}
