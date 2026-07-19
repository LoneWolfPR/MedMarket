package shipping

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

	"github.com/LoneWolfPR/MedMarket/backend/internal/httpx"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// Client is the struct that defines the properties for the adapter
type Client struct {
	client  *http.Client
	logger  *slog.Logger
	baseURL string
}

var _ outbound.ShippingClient = (*Client)(nil)

// NewShippingClientParams holds the parameters necessary to construct an instance
type NewShippingClientParams struct {
	Client  *http.Client
	Logger  *slog.Logger
	BaseURL string
}

// NewShippingClient constructs an instance of the adapter
func NewShippingClient(p NewShippingClientParams) (*Client, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	trimmedBaseURL := strings.TrimSpace(p.BaseURL)
	if trimmedBaseURL == "" {
		return nil, errors.New("base url is missing")
	}
	return &Client{
		client:  p.Client,
		logger:  p.Logger,
		baseURL: trimmedBaseURL,
	}, nil
}

type registerBody struct {
	TrackingID  string `json:"trackingId"`
	CallbackURL string `json:"callbackUrl"`
}

//nolint:revive // sentinel error
var ErrShippingRejected = errors.New("shipping registration rejected")

// RegisterWebhook reaches out to the shipping service and sets up a callback
// url for the system to notify when there are updates to a shipment attached
// to the provided tracking id
func (c *Client) RegisterWebhook(ctx context.Context, trackingID, callbackURL string) error {
	registerPath := c.baseURL + "/api/register-webhook"
	reqBody := registerBody{
		TrackingID:  trackingID,
		CallbackURL: callbackURL,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshaling json data: %w: %w", ErrShippingRejected, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error constructing request: %w: %w", ErrShippingRejected, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error registering webhook with shipping service: %w", err)
	}

	//nolint:errcheck // unnecessary
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		respError := fmt.Errorf(
			"failed to register webhook with shipping service with status %d: %s",
			resp.StatusCode, bodyBytes)
		if !httpx.IsRetryableStatus(resp.StatusCode) {
			return fmt.Errorf("%w: %w", ErrShippingRejected, respError)
		}
		return respError
	}

	c.logger.DebugContext(ctx, "webhook successfully registered", "tracking id", trackingID)
	return nil
}
