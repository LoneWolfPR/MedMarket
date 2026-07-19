package order

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/shared"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TaskQueue is the Temporal task queue the order worker polls and the
// starter targets in StartWorkflowOptions. Both sides must use this exact value.
const TaskQueue = "order"

//nolint:revive // these are self-explanatory
const (
	WorkflowName        = "OrderWorkflow"
	ShippingSignalName  = "shipping-event"
	ShippingStatusQuery = "shipping-status"
	maxOrderLifetime    = 30 * 24 * time.Hour
)

// ShippingAddress is representation of the address specific to
// the order workflow
type ShippingAddress struct {
	Street1 string
	Street2 string
	City    string
	State   string
	Zip     string
}

// Input holds the parameters necessary for placing an order
type Input struct {
	IdempotencyKey string
	OrderID        uuid.UUID
	PharmacyCode   string
	PharmacyItemID string
	ItemName       string
	RecipientName  string
	RecipientEmail string
	Qty            int
	Address        ShippingAddress
	WebhookBaseURL string
}

// ShippingStatus is a string type specific to the shipping statuses
type ShippingStatus string

//nolint:revive // self-explanatory
const (
	StatusPending        ShippingStatus = "pending" // internal status used for new orders
	StatusPickedUp       ShippingStatus = "picked_up"
	StatusInTransit      ShippingStatus = "in_transit"
	StatusOutForDelivery ShippingStatus = "out_for_delivery"
	StatusDelivered      ShippingStatus = "delivered"
)

var statusRank = []ShippingStatus{
	StatusPickedUp,
	StatusInTransit,
	StatusOutForDelivery,
	StatusDelivered,
}

// SignalPayload holds the fields that will come in from a shipping signal update
type SignalPayload struct {
	Status     ShippingStatus
	TrackingID string
	OccurredAt time.Time
}

// OrderWorkflowID is a helper to generate a standard workflow ID so
// that the right workflow can always be referenced
//
//nolint:revive // stutter is acceptable here
func OrderWorkflowID(orderID uuid.UUID) string {
	return "order-" + orderID.String()
}

// OrderWorkflow is a long running flow that handles placing an order and
// and tracking the shipping updates until delivered
//
//nolint:revive // Name of workflow is explicit for easy tracking in temporal
func OrderWorkflow(ctx workflow.Context, i Input) error {
	var a *Activities
	var status = StatusPending
	var orderStatus = ""
	ctx = shared.WithDefaultActivityOptions(ctx)
	logger := workflow.GetLogger(ctx)

	// Create a query handler for current shipping status
	queryHandlerErr := workflow.SetQueryHandler(ctx, ShippingStatusQuery, func() ShippingStatus {
		return status
	})
	if queryHandlerErr != nil {
		return fmt.Errorf("error creating query handler: %w", queryHandlerErr)
	}

	done := false

	// Place the order
	var placeOrderResult *PlaceOrderActivityResult
	placeOrderErr := workflow.ExecuteActivity(ctx, a.PlaceOrderActivity, i).Get(ctx, &placeOrderResult)
	if placeOrderErr != nil {
		return handlePlacementFailure(ctx, i, placeOrderErr)
	}

	// Update the order record
	updateErr := updateOrder(ctx, UpdateOrderActivityInput{
		ID:              i.OrderID,
		PharmacyOrderID: &placeOrderResult.PharmacyOrderID,
		TrackingID:      &placeOrderResult.TrackingID,
		Status:          ptr.To(order.StatusConfirmed),
	})
	if updateErr != nil {
		// log but don't fail. The order is placed, and we have the information needed to keep tracking
		logger.Error("error updating order record", "error", updateErr)
	}
	orderStatus = order.StatusConfirmed

	// Register the webhook with the shipping tracker
	registerInput := RegisterWebhookActivityInput{
		TrackingID:  placeOrderResult.TrackingID,
		CallbackURL: i.WebhookBaseURL + "/webhooks/shipping/" + i.OrderID.String(),
	}

	registerErr := workflow.ExecuteActivity(ctx, a.RegisterWebhookActivity, registerInput).Get(ctx, nil)
	if registerErr != nil {
		return fmt.Errorf("error registering webhook: %w", registerErr)
	}

	// Set up signal channel for shipping updates
	signalChannel := workflow.GetSignalChannel(ctx, ShippingSignalName)

	// reusable signal handler
	handleSignal := func(payload SignalPayload) {
		// Make sure the incoming status is not outdated
		if slices.Index(statusRank, payload.Status) > slices.Index(statusRank, status) {
			status = payload.Status
			newOrderStatus := mapToOrderStatus(status)
			if status == StatusDelivered {
				done = true
			}
			if newOrderStatus != orderStatus {
				err := updateOrder(ctx, UpdateOrderActivityInput{
					ID:     i.OrderID,
					Status: ptr.To(newOrderStatus),
				})
				if err != nil {
					// log but don't fail. failure to update the record is not a failure of shipping
					logger.Error("error updating order", "order id", i.OrderID, "error", err)
				}
				orderStatus = newOrderStatus
			}
			// Construct and send email message
			plainStatus := strings.ReplaceAll(string(status), "_", " ")
			msg := i.RecipientName + ",\r\n\r\n" +
				"The shipping status of your order for " + i.ItemName + " has changed:\r\n" +
				"The new shipping status is now " + plainStatus + ".\r\n\r\n" +
				"Sincerely,\r\n" +
				"The MedMarket Team"
			emailInput := EmailUpdateInput{
				To:      i.RecipientEmail,
				Message: msg,
			}
			emailErr := workflow.ExecuteActivity(ctx, a.SendEmailUpdate, emailInput).Get(ctx, nil)
			if emailErr != nil {
				// don't fail the flow just because an email fails to send
				logger.Error("error sending email notification", "error", emailErr)
			}
		}
	}

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(signalChannel, func(c workflow.ReceiveChannel, more bool) {
		var payload SignalPayload
		c.Receive(ctx, &payload)
		handleSignal(payload)
	})

	// Setup a timer so the loop can't run infinitely
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	lifetimeTimer := workflow.NewTimer(timerCtx, maxOrderLifetime)
	selector.AddFuture(lifetimeTimer, func(workflow.Future) {
		done = true
		logger.Info("order lifetime elapsed; closing", "order id", i.OrderID)
	})
	for !done {
		selector.Select(ctx)
	}

	// drains any remaining signals if the workflow times out
	var payload SignalPayload
	for signalChannel.ReceiveAsync(&payload) {
		handleSignal(payload)
	}
	cancelTimer()
	return nil
}

func mapToOrderStatus(status ShippingStatus) string {
	switch status {
	case StatusPickedUp, StatusInTransit, StatusOutForDelivery:
		return order.StatusShipped
	case StatusDelivered:
		return order.StatusDelivered
	default:
		return order.StatusConfirmed
	}
}

func updateOrder(ctx workflow.Context, i UpdateOrderActivityInput) error {
	var a *Activities
	return workflow.ExecuteActivity(ctx, a.UpdateOrderActivity, i).Get(ctx, nil)
}

func handlePlacementFailure(
	ctx workflow.Context, i Input, err error) error {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		switch appErr.Type() {
		case string(outbound.KindRejected):
			// definitely not placed. Update status to failed
			updateErr := updateOrder(ctx, UpdateOrderActivityInput{
				ID:     i.OrderID,
				Status: ptr.To(order.StatusFailed),
			})
			if updateErr != nil {
				return fmt.Errorf(
					"order placement rejected with error updating order record: %w: %w",
					err, updateErr,
				)
			}
			return fmt.Errorf("order placement rejected: %w", err)
		case string(outbound.KindOutcomeUnknown):
			// outcome is unknown. Log, but don't update status
			return fmt.Errorf("error placing order. completion unclear: %w", err)
		default:
			// Some other non-retryable error. record as a failure
			updateErr := updateOrder(ctx, UpdateOrderActivityInput{
				ID:     i.OrderID,
				Status: ptr.To(order.StatusFailed),
			})
			if updateErr != nil {
				return fmt.Errorf(
					"unknown unretryable error when placing order with error updating order record: %w: %w",
					err, updateErr,
				)
			}
			return fmt.Errorf("unknown unretryable error when placing order with error updating order record: %w", err)
		}
	}
	return fmt.Errorf("failed to place order: %w", err)
}
