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
	client  *http.Client
	logger  *slog.Logger
	id      uuid.UUID
	secret  string
	baseURL string
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

type graphQLRequest struct {
	Query     string          `json:"query"`
	Variables searchVariables `json:"variables"`
}

type searchVariables struct {
	Name     string `json:"name"`
	Strength string `json:"strength"`
}

type graphQLResponse struct {
	Data   responseData   `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type responseData struct {
	Medications []medicationItem `json:"medications"`
}

type medicationItem struct {
	Code      string `json:"code"`
	UnitPrice money  `json:"unitPrice"`
}

type money struct {
	Amount string `json:"amount"`
}

type graphQLError struct {
	Message string `json:"message"`
}

const searchQuery = `query Search($name: String!, $strength: String) {
	medications(name: $name, strength: $strength) {
		code
		unitPrice { amount }
	}
}`

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
		client:  p.Client,
		logger:  p.Logger,
		secret:  p.Secret,
		baseURL: p.BaseURL,
		id:      p.ID,
	}, nil
}

// Search queries the api for a list of prices for a specific medication by name and strength
func (pb *PharmacyB) Search(
	ctx context.Context,
	c pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	searchPath := pb.baseURL + "/graphql"
	priceQuoteList := []pharmacy.PriceQuote{}

	searchVars := searchVariables{
		Name:     c.MedName(),
		Strength: c.MedStrength().String(),
	}

	gqlQuery := graphQLRequest{
		Query:     searchQuery,
		Variables: searchVars,
	}

	jsonData, err := json.Marshal(gqlQuery)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchPath, bytes.NewBuffer(jsonData))
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

	var gqlResp graphQLResponse
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
