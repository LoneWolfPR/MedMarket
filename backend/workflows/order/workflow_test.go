package order_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	orderwf "github.com/LoneWolfPR/MedMarket/backend/workflows/order"
)

// newWorkflowEnv builds a Temporal test environment with the activity struct
// registered so the workflow's activity references resolve by name. Every test
// mocks the activities with OnActivity, so the real (nil-dependency)
// implementations never run.
func newWorkflowEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(&orderwf.Activities{})
	t.Cleanup(func() { env.AssertExpectations(t) })
	return env
}

// okPlaceResult is the standard successful placement result.
func okPlaceResult() *orderwf.PlaceOrderActivityResult {
	return &orderwf.PlaceOrderActivityResult{
		PharmacyOrderID: "pharm-order-1",
		TrackingID:      "trk-1",
		NewTotalCents:   1299,
		OrderStatus:     "placed",
	}
}

func TestOrderWorkflow_ZeroDollarSkipsAuthorizeAndCapture(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	// Authorize/Capture are mocked only to record whether they run; a $0 order
	// must never touch the payment gateway.
	authorizeCalled, captureCalled := false, false
	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).
		Return(func(context.Context, orderwf.AuthorizeActivityInput) (string, error) {
			authorizeCalled = true
			return "pi_x", nil
		}).Maybe()
	env.OnActivity(a.CapturePaymentActivity, mock.Anything, mock.Anything).
		Return(func(context.Context, orderwf.CaptureActivityInput) (string, error) {
			captureCalled = true
			return "pi_x", nil
		}).Maybe()

	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).Return(okPlaceResult(), nil)
	env.OnActivity(a.RegisterWebhookActivity, mock.Anything, mock.Anything).Return(nil)

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(0))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.False(t, authorizeCalled, "a $0 order must not authorize")
	assert.False(t, captureCalled, "a $0 order must not capture")

	// The lone update is the confirmed transition, and price paid is a real $0
	// (set, not nil) — a fully-insured order settled at zero.
	require.Len(t, updates, 1)
	assert.Equal(t, order.StatusConfirmed, *updates[0].Status)
	require.NotNil(t, updates[0].PricePaidCents)
	assert.Equal(t, int64(0), *updates[0].PricePaidCents)
}

func TestOrderWorkflow_PaidHappyPathAuthorizesCapturesAndRecordsPrice(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	var gotAuth orderwf.AuthorizeActivityInput
	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.AuthorizeActivityInput) (string, error) {
			gotAuth = i
			return "pi_123", nil
		})

	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).Return(okPlaceResult(), nil)

	var gotCapture orderwf.CaptureActivityInput
	env.OnActivity(a.CapturePaymentActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.CaptureActivityInput) (string, error) {
			gotCapture = i
			return "pi_123", nil
		})

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})
	env.OnActivity(a.RegisterWebhookActivity, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(1299))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	assert.Equal(t, int64(1299), gotAuth.AmountCents)
	assert.Equal(t, "pm_card_visa", gotAuth.PaymentMethod)
	assert.Equal(t, "pi_123", gotCapture.AuthID)
	assert.Equal(t, int64(1299), gotCapture.AmountCents)

	require.Len(t, updates, 1)
	assert.Equal(t, order.StatusConfirmed, *updates[0].Status)
	require.NotNil(t, updates[0].PricePaidCents)
	assert.Equal(t, int64(1299), *updates[0].PricePaidCents)
}

func TestOrderWorkflow_AuthorizeFailureFailsWorkflowAndMarksFailed(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	// A card decline surfaces as a non-retryable ApplicationError.
	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).
		Return("", temporal.NewNonRetryableApplicationError("declined", "declined", errBoom))

	placeCalled := false
	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).
		Return(func(context.Context, orderwf.Input) (*orderwf.PlaceOrderActivityResult, error) {
			placeCalled = true
			return okPlaceResult(), nil
		}).Maybe()

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(1299))

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.False(t, placeCalled, "a failed authorization must never place the order")

	require.Len(t, updates, 1)
	assert.Equal(t, order.StatusFailed, *updates[0].Status)
}

func TestOrderWorkflow_CaptureFailureProceedsAndLeavesPriceUnset(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).Return("pi_123", nil)
	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).Return(okPlaceResult(), nil)

	// A terminal capture failure. The order is already placed, so the workflow
	// logs and proceeds rather than voiding or failing.
	env.OnActivity(a.CapturePaymentActivity, mock.Anything, mock.Anything).
		Return("", temporal.NewNonRetryableApplicationError("capture failed", "unexpected", errBoom))

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})
	env.OnActivity(a.RegisterWebhookActivity, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(1299))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "a capture failure must not fail the workflow")

	// Confirmed, but price paid is left nil — the placed-but-not-captured signal.
	require.Len(t, updates, 1)
	assert.Equal(t, order.StatusConfirmed, *updates[0].Status)
	assert.Nil(t, updates[0].PricePaidCents, "an uncaptured order must record no price paid")
}

func TestOrderWorkflow_PlacementRejectedVoidsAuthAndMarksFailed(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).Return("pi_123", nil)
	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).
		Return(nil, temporal.NewNonRetryableApplicationError(
			"rejected", string(outbound.KindRejected), errBoom))

	var voidedAuthID string
	env.OnActivity(a.VoidPaymentActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, authID string) error {
			voidedAuthID = authID
			return nil
		})

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(1299))

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Equal(t, "pi_123", voidedAuthID, "a rejected placement must void the held auth")

	require.Len(t, updates, 1)
	assert.Equal(t, order.StatusFailed, *updates[0].Status)
}

func TestOrderWorkflow_PlacementOutcomeUnknownParksWithoutVoidOrUpdate(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	env.OnActivity(a.AuthorizePaymentActivity, mock.Anything, mock.Anything).Return("pi_123", nil)
	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).
		Return(nil, temporal.NewNonRetryableApplicationError(
			"outcome unknown", string(outbound.KindOutcomeUnknown), errBoom))

	// Both are mocked only to record non-invocation: parking must leave the auth
	// standing (no void) and the order untouched (no status write).
	voidCalled, updateCalled := false, false
	env.OnActivity(a.VoidPaymentActivity, mock.Anything, mock.Anything).
		Return(func(context.Context, string) error {
			voidCalled = true
			return nil
		}).Maybe()
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(context.Context, orderwf.UpdateOrderActivityInput) error {
			updateCalled = true
			return nil
		}).Maybe()

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(1299))

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.False(t, voidCalled, "an unknown outcome must leave the auth standing")
	assert.False(t, updateCalled, "an unknown outcome must not commit a terminal status")
}

func TestOrderWorkflow_ShippingSignalsAdvanceStatusMonotonically(t *testing.T) {
	var a *orderwf.Activities
	env := newWorkflowEnv(t)

	// $0 order to keep the focus on the shipping loop (no payment activities).
	env.OnActivity(a.PlaceOrderActivity, mock.Anything, mock.Anything).Return(okPlaceResult(), nil)
	env.OnActivity(a.RegisterWebhookActivity, mock.Anything, mock.Anything).Return(nil)

	var updates []orderwf.UpdateOrderActivityInput
	env.OnActivity(a.UpdateOrderActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, i orderwf.UpdateOrderActivityInput) error {
			updates = append(updates, i)
			return nil
		})

	emailCount := 0
	env.OnActivity(a.SendEmailUpdate, mock.Anything, mock.Anything).
		Return(func(context.Context, orderwf.EmailUpdateInput) error {
			emailCount++
			return nil
		})

	// Deliver signals out of order: picked_up, then out_for_delivery, then a stale
	// in_transit (must be ignored by the monotonic guard), then delivered.
	signal := func(status orderwf.ShippingStatus) {
		env.SignalWorkflow(orderwf.ShippingSignalName, orderwf.SignalPayload{
			Status:     status,
			TrackingID: "trk-1",
			OccurredAt: time.Now(),
		})
	}
	env.RegisterDelayedCallback(func() { signal(orderwf.StatusPickedUp) }, time.Millisecond)
	env.RegisterDelayedCallback(func() { signal(orderwf.StatusOutForDelivery) }, 2*time.Millisecond)
	env.RegisterDelayedCallback(func() { signal(orderwf.StatusInTransit) }, 3*time.Millisecond)
	env.RegisterDelayedCallback(func() { signal(orderwf.StatusDelivered) }, 4*time.Millisecond)

	env.ExecuteWorkflow(orderwf.OrderWorkflow, validInput(0))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Order-status writes: confirmed (initial), shipped (picked_up), delivered.
	// out_for_delivery also maps to "shipped" (no new write); in_transit is stale.
	var statuses []string
	for _, u := range updates {
		if u.Status != nil {
			statuses = append(statuses, *u.Status)
		}
	}
	assert.Equal(t, []string{order.StatusConfirmed, order.StatusShipped, order.StatusDelivered}, statuses)

	// One email per accepted advance: picked_up, out_for_delivery, delivered. The
	// stale in_transit is dropped, so a fourth email would mean a guard failure.
	assert.Equal(t, 3, emailCount)
}
