package stripe_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v86"

	stripeadapter "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/stripe"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const (
	testSecret = "sk_test_123"
	testPIID   = "pi_test_123"
)

// nopLogger silences stripe-go's default stderr logging during tests.
type nopLogger struct{}

func (nopLogger) Debugf(string, ...interface{}) {}
func (nopLogger) Errorf(string, ...interface{}) {}
func (nopLogger) Infof(string, ...interface{})  {}
func (nopLogger) Warnf(string, ...interface{})  {}

// newAdapter wires a PaymentGateway to a stripe client that talks to the stub
// server instead of the real Stripe API. Retries are disabled so error-status
// responses hit the handler exactly once and don't add backoff delay.
func newAdapter(t *testing.T, handler http.HandlerFunc) *stripeadapter.PaymentGateway {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	backends := stripe.NewBackendsWithConfig(&stripe.BackendConfig{
		URL:               stripe.String(srv.URL),
		HTTPClient:        srv.Client(),
		MaxNetworkRetries: stripe.Int64(0),
		LeveledLogger:     nopLogger{},
	})
	sc := stripe.NewClient(testSecret, stripe.WithBackends(backends))

	g, err := stripeadapter.NewPaymentGateway(stripeadapter.NewPaymentGatewayParams{
		Client: sc,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return g
}

// okJSON replies 200 with a minimal PaymentIntent body carrying testPIID.
func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"` + testPIID + `","object":"payment_intent"}`))
}

// errorJSON replies with a Stripe-shaped error body of the given type. The
// classification the adapter makes keys off error.type, not the HTTP status.
func errorJSON(status int, errType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":{"type":"` + errType + `","message":"test error"}}`))
	}
}

func money(t *testing.T, cents int64) shared.Money {
	t.Helper()
	m, err := shared.NewMoneyFromCents(cents)
	require.NoError(t, err)
	return m
}

func validAuthInput(t *testing.T) outbound.AuthorizeInput {
	t.Helper()
	return outbound.AuthorizeInput{
		OrderID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Amount:        money(t, 1299),
		PaymentMethod: "pm_card_visa",
	}
}

func TestNewPaymentGateway_Guards(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	client := stripe.NewClient(testSecret)

	t.Run("nil client", func(t *testing.T) {
		_, err := stripeadapter.NewPaymentGateway(stripeadapter.NewPaymentGatewayParams{Logger: logger})
		require.Error(t, err)
	})
	t.Run("nil logger", func(t *testing.T) {
		_, err := stripeadapter.NewPaymentGateway(stripeadapter.NewPaymentGatewayParams{Client: client})
		require.Error(t, err)
	})
	t.Run("valid", func(t *testing.T) {
		_, err := stripeadapter.NewPaymentGateway(stripeadapter.NewPaymentGatewayParams{
			Client: client,
			Logger: logger,
		})
		require.NoError(t, err)
	})
}

func TestAuthorize_HappyPathSendsCorrectRequest(t *testing.T) {
	in := validAuthInput(t)
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// Assert the adapter's own request translation: endpoint, the create
		// params mapped onto Stripe's form fields, and the idempotency key.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/payment_intents", r.URL.Path)
		assert.Equal(t, "order-"+in.OrderID.String()+":authorize", r.Header.Get("Idempotency-Key"))

		require.NoError(t, r.ParseForm())
		assert.Equal(t, strconv.FormatInt(in.Amount.Cents(), 10), r.PostForm.Get("amount"))
		assert.Equal(t, "usd", r.PostForm.Get("currency"))
		assert.Equal(t, in.PaymentMethod, r.PostForm.Get("payment_method"))
		assert.Equal(t, "manual", r.PostForm.Get("capture_method"))
		assert.Equal(t, "true", r.PostForm.Get("confirm"))
		assert.Equal(t, in.OrderID.String(), r.PostForm.Get("metadata[order_id]"))
		// Pinned to card only, so Stripe never enables redirect-based payment
		// methods that would demand a return_url this server-side flow can't give.
		assert.Equal(t, "card", r.PostForm.Get("payment_method_types[0]"))

		okJSON(w, r)
	})

	res, err := adapter.Authorize(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, testPIID, res.AuthorizationID)
}

func TestAuthorize_ErrorClassification(t *testing.T) {
	cases := map[string]struct {
		status    int
		errType   string
		wantKind  outbound.PaymentErrorKind
		wantTyped bool // true = a *PaymentError (terminal); false = plain (retryable)
	}{
		"card decline is terminal declined":      {402, "card_error", outbound.PaymentKindDeclined, true},
		"invalid request is terminal unexpected": {400, "invalid_request_error", outbound.PaymentKindUnexpected, true},
		"idempotency is terminal unexpected":     {400, "idempotency_error", outbound.PaymentKindUnexpected, true},
		"api error is retryable plain":           {500, "api_error", "", false},
		"rate limit is retryable plain":          {429, "rate_limit", "", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			adapter := newAdapter(t, errorJSON(tc.status, tc.errType))
			_, err := adapter.Authorize(context.Background(), validAuthInput(t))
			require.Error(t, err)

			var pe *outbound.PaymentError
			if tc.wantTyped {
				require.ErrorAs(t, err, &pe)
				assert.Equal(t, tc.wantKind, pe.Kind)
			} else {
				assert.False(t, errors.As(err, &pe), "retryable errors must stay plain, not *PaymentError")
			}
		})
	}
}

func TestCapture_HappyPathSendsCorrectRequest(t *testing.T) {
	amount := money(t, 1299)
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/payment_intents/"+testPIID+"/capture", r.URL.Path)
		assert.Equal(t, testPIID+":capture", r.Header.Get("Idempotency-Key"))

		require.NoError(t, r.ParseForm())
		assert.Equal(t, strconv.FormatInt(amount.Cents(), 10), r.PostForm.Get("amount_to_capture"))

		okJSON(w, r)
	})

	res, err := adapter.Capture(context.Background(), testPIID, amount)
	require.NoError(t, err)
	assert.Equal(t, testPIID, res.AuthorizationID)
}

func TestCapture_ErrorClassification(t *testing.T) {
	cases := map[string]struct {
		status    int
		errType   string
		wantKind  outbound.PaymentErrorKind
		wantTyped bool
	}{
		"invalid request is terminal unexpected": {400, "invalid_request_error", outbound.PaymentKindUnexpected, true},
		// Capture deliberately drops the card branch: a card_error at capture
		// time (unreachable in practice) falls through to retryable/plain.
		"card error is retryable plain": {402, "card_error", "", false},
		"api error is retryable plain":  {500, "api_error", "", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			adapter := newAdapter(t, errorJSON(tc.status, tc.errType))
			_, err := adapter.Capture(context.Background(), testPIID, money(t, 1299))
			require.Error(t, err)

			var pe *outbound.PaymentError
			if tc.wantTyped {
				require.ErrorAs(t, err, &pe)
				assert.Equal(t, tc.wantKind, pe.Kind)
			} else {
				assert.False(t, errors.As(err, &pe))
			}
		})
	}
}

func TestVoid_HappyPathSendsCorrectRequest(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/payment_intents/"+testPIID+"/cancel", r.URL.Path)
		assert.Equal(t, testPIID+":void", r.Header.Get("Idempotency-Key"))
		okJSON(w, r)
	})

	err := adapter.Void(context.Background(), testPIID)
	require.NoError(t, err)
}

func TestVoid_ErrorStaysPlain(t *testing.T) {
	// Void never classifies; every failure is a plain (retryable) error.
	adapter := newAdapter(t, errorJSON(400, "invalid_request_error"))
	err := adapter.Void(context.Background(), testPIID)
	require.Error(t, err)

	var pe *outbound.PaymentError
	assert.False(t, errors.As(err, &pe))
}
