//go:build smoke

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSmokeJourney walks the full customer journey against a *running* stack
// (task up): register -> login -> upload -> search -> order -> status. Unlike the
// boot test, nothing here is faked — it exercises real Postgres, Temporal, MinIO,
// the mock pharmacies, and Stripe test mode through Traefik. It is the end-to-end
// proof that the deployed compose topology actually works together.
//
// It needs the stack up and STRIPE_SECRET_KEY set (see the .env / README). The
// base URL defaults to Traefik on localhost; override with SMOKE_BASE_URL.
func TestSmokeJourney(t *testing.T) {
	base := os.Getenv("SMOKE_BASE_URL")
	if base == "" {
		base = "http://localhost"
	}
	c := &smokeClient{t: t, base: base}

	email := "smoke-" + uuid.NewString() + "@example.com"
	const password = "Test1234!"

	// register — an address is required to order (it becomes the shipping address)
	status, _ := c.postJSON("/api/auth/register", "", map[string]any{
		"firstName": "Smoke",
		"lastName":  "Test",
		"email":     email,
		"password":  password,
		"address": map[string]any{
			"street1": "1 Main St",
			"city":    "Anytown",
			"state":   "CA",
			"zip":     "90001",
		},
	})
	require.Equal(t, http.StatusCreated, status, "register")

	// login -> token
	var login struct {
		Token string `json:"token"`
	}
	status, body := c.postJSON("/api/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, status, "login")
	require.NoError(t, json.Unmarshal(body, &login))
	require.NotEmpty(t, login.Token)
	token := login.Token

	// upload a prescription for a med both mocks stock, so the search returns quotes
	var rx struct {
		ID string `json:"id"`
	}
	status, body = c.upload(token, map[string]string{
		"physicianName": "Dr. Smoke",
		"medName":       "Atorvastatin",
		"strengthValue": "20",
		"strengthUnit":  "mg",
		"quantity":      "30",
	})
	require.Equal(t, http.StatusCreated, status, "upload: %s", body)
	require.NoError(t, json.Unmarshal(body, &rx))
	require.NotEmpty(t, rx.ID)

	// search -> quotes, cheapest first
	var quotes []struct {
		OfferID    string `json:"offerId"`
		TotalCents int64  `json:"totalCents"`
	}
	status, body = c.postJSON("/api/prescriptions/"+rx.ID+"/search", token, nil)
	require.Equal(t, http.StatusOK, status, "search: %s", body)
	require.NoError(t, json.Unmarshal(body, &quotes))
	require.NotEmpty(t, quotes, "expected at least one quote")
	offerID := quotes[0].OfferID
	require.NotEmpty(t, offerID)

	// order the cheapest offer with a Stripe test card that always succeeds
	var order struct {
		OrderID string `json:"orderId"`
		Status  string `json:"status"`
	}
	status, body = c.postJSON("/api/orders", token, map[string]any{
		"offerId":       offerID,
		"paymentMethod": "pm_card_visa",
	})
	require.Equal(t, http.StatusCreated, status, "create order: %s", body)
	require.NoError(t, json.Unmarshal(body, &order))
	require.NotEmpty(t, order.OrderID)

	// the saga starts the order at "placed", then the prologue (authorize -> place
	// -> capture) advances it to "confirmed". Poll until it leaves "placed"; a
	// terminal "confirmed" proves payment + pharmacy placement + the DB write all
	// worked end-to-end.
	final := c.pollOrderStatus(token, order.OrderID, 30*time.Second)
	assert.Equal(t, "confirmed", final, "order should reach confirmed after the saga prologue")
}

// pollOrderStatus polls the status endpoint until the coarse status leaves
// "placed" or the timeout elapses, returning the last status seen.
func (c *smokeClient) pollOrderStatus(token, orderID string, timeout time.Duration) string {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		status, body := c.get("/api/orders/"+orderID+"/status", token)
		require.Equal(c.t, http.StatusOK, status, "get status: %s", body)
		var s struct {
			Status string `json:"status"`
		}
		require.NoError(c.t, json.Unmarshal(body, &s))
		last = s.Status
		if last != "" && last != "placed" {
			return last
		}
		time.Sleep(500 * time.Millisecond)
	}
	return last
}

// smokeClient is a thin HTTP helper bound to a base URL and the test.
type smokeClient struct {
	t    *testing.T
	base string
}

func (c *smokeClient) get(path, token string) (int, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	require.NoError(c.t, err)
	return c.do(req, token)
}

func (c *smokeClient) postJSON(path, token string, jsonBody any) (int, []byte) {
	c.t.Helper()
	var reader io.Reader
	if jsonBody != nil {
		buf, err := json.Marshal(jsonBody)
		require.NoError(c.t, err)
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, reader)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, token)
}

// upload builds a multipart/form-data request with a small placeholder PDF as the
// document plus the given text fields.
func (c *smokeClient) upload(token string, fields map[string]string) (int, []byte) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	file, err := mw.CreateFormFile("document", "prescription.pdf")
	require.NoError(c.t, err)
	_, err = file.Write([]byte("%PDF-1.4\n% smoke-test placeholder\n"))
	require.NoError(c.t, err)

	for k, v := range fields {
		require.NoError(c.t, mw.WriteField(k, v))
	}
	require.NoError(c.t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, c.base+"/api/prescriptions/upload", &buf)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.do(req, token)
}

func (c *smokeClient) do(req *http.Request, token string) (int, []byte) {
	c.t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { require.NoError(c.t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return resp.StatusCode, body
}
