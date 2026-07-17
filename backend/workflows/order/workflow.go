package order

import (
	"time"

	"github.com/google/uuid"
)

// TaskQueue is the Temporal task queue the order worker polls and the
// starter targets in StartWorkflowOptions. Both sides must use this exact value.
const TaskQueue = "order"

//nolint:revive // these are self-explanatory
const (
	WorkflowName        = "OrderWorkflow"
	ShippingSignalName  = "shipping-event"
	ShippingStatusQuery = "shipping-status"
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
	RecipientName  string
	Qty            int
	Address        ShippingAddress
}

// ShippingStatus is a string type specific to the shipping statuses
type ShippingStatus string

//nolint:revive // self-explanatory
const (
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
