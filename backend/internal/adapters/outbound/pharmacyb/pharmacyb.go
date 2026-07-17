package pharmacyb

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
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// PharmacyB is the adapter for making queries against Pharmacy B
type PharmacyB struct {
	client   *http.Client
	logger   *slog.Logger
	id       uuid.UUID
	secret   string
	endpoint string
}

var _ outbound.PharmacyClient = (*PharmacyB)(nil)

// NewPharmacyBParams are the input params necessary for the adapter
type NewPharmacyBParams struct {
	Client  *http.Client
	Logger  *slog.Logger
	ID      uuid.UUID
	Secret  string
	BaseURL string
}

type money struct {
	Amount string `json:"amount"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// NewPharmacyB constructs a new instance of the PharmacyB adapter
func NewPharmacyB(p NewPharmacyBParams) (*PharmacyB, error) {
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
	return &PharmacyB{
		client:   p.Client,
		logger:   p.Logger,
		secret:   p.Secret,
		endpoint: strings.TrimRight(p.BaseURL, "/") + "/graphql",
		id:       p.ID,
	}, nil
}

type graphQLSearchRequest struct {
	Query     string          `json:"query"`
	Variables searchVariables `json:"variables"`
}

type searchVariables struct {
	Name     string `json:"name"`
	Strength string `json:"strength"`
}

type graphQLSearchResponse struct {
	Data   searchResponseData `json:"data"`
	Errors []graphQLError     `json:"errors"`
}

type searchResponseData struct {
	Medications []medicationItem `json:"medications"`
}

type medicationItem struct {
	Code      string `json:"code"`
	UnitPrice money  `json:"unitPrice"`
}

const searchQuery = `query Search($name: String!, $strength: String) {
	medications(name: $name, strength: $strength) {
		code
		unitPrice { amount }
	}
}`

// Search queries the api for a list of prices for a specific medication by name and strength
func (pb *PharmacyB) Search(
	ctx context.Context,
	c pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	priceQuoteList := []pharmacy.PriceQuote{}

	searchVars := searchVariables{
		Name:     c.MedName(),
		Strength: c.MedStrength().String(),
	}

	gqlQuery := graphQLSearchRequest{
		Query:     searchQuery,
		Variables: searchVars,
	}

	jsonData, err := json.Marshal(gqlQuery)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pb.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error constructing request: %w", err)
	}
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pb.secret)

	resp, err := pb.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error searching: %w", err)
	}

	//nolint:errcheck // unnecessary
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pharmacy B search failed: status %d: %s", resp.StatusCode, bodyBytes)
	}

	var gqlResp graphQLSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	for _, result := range gqlResp.Data.Medications {
		trimmedCode := strings.TrimSpace(result.Code)
		if trimmedCode == "" {
			pb.logger.WarnContext(ctx, "empty code on item")
			continue
		}

		priceCents, err := shared.NewMoneyFromDollars(result.UnitPrice.Amount)
		if err != nil {
			pb.logger.WarnContext(ctx, "error reading price on item", "code", trimmedCode, "error", err)
			continue
		}

		quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
			Price:          priceCents,
			PharmacyID:     pb.id,
			PharmacyItemID: trimmedCode,
		})
		if err != nil {
			pb.logger.WarnContext(ctx, "error creating quote", "error", err)
			continue
		}
		priceQuoteList = append(priceQuoteList, quote)
	}

	if len(gqlResp.Errors) > 0 {
		messages := []string{}
		for _, gqlErr := range gqlResp.Errors {
			messages = append(messages, gqlErr.Message)
		}
		messagesStr := strings.Join(messages, "; ")
		if len(priceQuoteList) == 0 {
			return nil, fmt.Errorf("graphql errors returned: %s", messagesStr)
		}
		pb.logger.WarnContext(ctx, "errors returned from graphql query", "errors", messagesStr)
	}

	return priceQuoteList, nil
}

type graphQLOrderRequest struct {
	Mutation  string         `json:"query"`
	Variables orderVariables `json:"variables"`
}

type orderVariables struct {
	Input graphQLOrderInput `json:"input"`
}

type graphQLOrderInputRecipient struct {
	Name       string `json:"name"`
	Line1      string `json:"line1"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
}

type graphQLOrderInput struct {
	Code      string                     `json:"code"`
	Count     int                        `json:"count"`
	Recipient graphQLOrderInputRecipient `json:"recipient"`
}

type graphQLOrderResponse struct {
	Data   orderResponseData `json:"data"`
	Errors []graphQLError    `json:"errors"`
}

type orderResponseData struct {
	PlaceOrder *placeOrderResponse `json:"placeOrder,omitempty"`
}

type placeOrderResponse struct {
	Reference  string `json:"reference"`
	Tracking   string `json:"tracking"`
	GrandTotal money  `json:"grandTotal"`
	State      string `json:"state"`
}

const orderMutation = `mutation PlaceOrder($input: OrderInput!) {
	placeOrder(input: $input) {
		reference
		tracking
		state
		grandTotal { amount }
	}
}`

// PlaceOrder sends an order request to the api
func (pb *PharmacyB) PlaceOrder(
	ctx context.Context,
	i pharmacy.OrderInput,
) (pharmacy.OrderResult, error) {
	addr := i.ShippingAddress()
	line1Val := addr.Street1
	if addr.Street2 != "" {
		line1Val += ", " + addr.Street2
	}
	orderVars := orderVariables{
		Input: graphQLOrderInput{
			Code:  i.PharmacyItemID(),
			Count: i.Qty(),
			Recipient: graphQLOrderInputRecipient{
				Name:       i.RecipientName(),
				Line1:      line1Val,
				City:       addr.City,
				State:      addr.State,
				PostalCode: addr.Zip,
			},
		},
	}

	gqlMutation := graphQLOrderRequest{
		Mutation:  orderMutation,
		Variables: orderVars,
	}

	jsonData, err := json.Marshal(gqlMutation)
	if err != nil {
		newErr := outbound.NewPlaceOrderError(
			outbound.KindRejected,
			fmt.Errorf("error marshaling json data: %w", err),
		)
		return pharmacy.OrderResult{}, newErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pb.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		newErr := outbound.NewPlaceOrderError(
			outbound.KindRejected,
			fmt.Errorf("error constructing request: %w", err),
		)
		return pharmacy.OrderResult{}, newErr
	}
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pb.secret)

	resp, err := pb.client.Do(req)
	if err != nil {
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("error placing an order: %w", err),
		)
		return pharmacy.OrderResult{}, respError
	}

	//nolint:errcheck // unnecessary
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errKind := outbound.KindRejected
		if resp.StatusCode >= 500 {
			errKind = outbound.KindOutcomeUnknown
		}
		respError := outbound.NewPlaceOrderError(
			errKind,
			fmt.Errorf(
				"pharmacy B failed to place order with status %d: %s",
				resp.StatusCode, bodyBytes),
		)
		return pharmacy.OrderResult{}, respError
	}

	var gqlResp graphQLOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("error decoding response: %w", err),
		)
		return pharmacy.OrderResult{}, respError
	}

	if len(gqlResp.Errors) > 0 {
		messages := []string{}
		for _, gqlErr := range gqlResp.Errors {
			messages = append(messages, gqlErr.Message)
		}
		messagesStr := strings.Join(messages, "; ")
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("graphql errors returned: %s", messagesStr),
		)
		return pharmacy.OrderResult{}, respError
	}

	orderData := gqlResp.Data.PlaceOrder
	if orderData == nil {
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("missing response data: %v", gqlResp),
		)
		return pharmacy.OrderResult{}, respError
	}

	newTotal, err := shared.NewMoneyFromDollars(orderData.GrandTotal.Amount)
	if err != nil {
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("error processing returned amount: %w", err),
		)
		return pharmacy.OrderResult{}, respError
	}

	result, err := pharmacy.NewOrderResult(pharmacy.NewOrderResultParams{
		PharmacyOrderID: orderData.Reference,
		TrackingID:      orderData.Tracking,
		NewTotal:        newTotal,
		OrderStatus:     orderData.State,
	})

	if err != nil {
		respError := outbound.NewPlaceOrderError(
			outbound.KindOutcomeUnknown,
			fmt.Errorf("error parsing response: %w", err),
		)
		return pharmacy.OrderResult{}, respError
	}

	return result, nil
}
