package pharmacy

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// OrderResult is a VO type that contains the information
// about the result of placing an order
type OrderResult struct {
	pharmacyOrderID string
	trackingID      string
	newTotal        shared.Money
	orderStatus     string
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingPharmOrderID = errors.New("pharmacy order id is missing")
	ErrMissingTrackingID   = errors.New("tracking id is missing")
	ErrMissingOrderStatus  = errors.New("order status is missing")
)

// NewOrderResultParams holds the input params necessary to construct an instance
type NewOrderResultParams struct {
	PharmacyOrderID string
	TrackingID      string
	NewTotal        shared.Money
	OrderStatus     string
}

// NewOrderResult constructs and instance
func NewOrderResult(p NewOrderResultParams) (OrderResult, error) {
	trimmedPharmOrderID := strings.TrimSpace(p.PharmacyOrderID)
	trimmedTrackingID := strings.TrimSpace(p.TrackingID)
	trimmedStatus := strings.TrimSpace(p.OrderStatus)
	if trimmedPharmOrderID == "" {
		return OrderResult{}, ErrMissingPharmOrderID
	}
	if trimmedTrackingID == "" {
		return OrderResult{}, ErrMissingTrackingID
	}
	if trimmedStatus == "" {
		return OrderResult{}, ErrMissingOrderStatus
	}
	return OrderResult{
		pharmacyOrderID: trimmedPharmOrderID,
		trackingID:      trimmedTrackingID,
		newTotal:        p.NewTotal,
		orderStatus:     trimmedStatus,
	}, nil
}

// PharmacyOrderID is a getter
func (r OrderResult) PharmacyOrderID() string {
	return r.pharmacyOrderID
}

// TrackingID is a getter
func (r OrderResult) TrackingID() string {
	return r.trackingID
}

// NewTotal is a getter
func (r OrderResult) NewTotal() shared.Money {
	return r.newTotal
}

// OrderStatus is a getter
func (r OrderResult) OrderStatus() string {
	return r.orderStatus
}
