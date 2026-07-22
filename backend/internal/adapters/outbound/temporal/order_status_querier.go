package temporal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/order"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// OrderStatusQuerier is the adapter for querying the shipping status of an order
type OrderStatusQuerier struct {
	client client.Client
	logger *slog.Logger
}

var _ outbound.OrderStatusQuerier = (*OrderStatusQuerier)(nil)

// NewOrderStatusQuerierParams holds the parameters necessary to construct the adapter
type NewOrderStatusQuerierParams struct {
	Client client.Client
	Logger *slog.Logger
}

// NewOrderStatusQuerier constructs and instance of the adapter
func NewOrderStatusQuerier(p NewOrderStatusQuerierParams) (*OrderStatusQuerier, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &OrderStatusQuerier{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// QueryShippingStatus queries temporal for a running workflow and returns the status of one that is found
func (q *OrderStatusQuerier) QueryShippingStatus(ctx context.Context, orderID uuid.UUID) (string, error) {
	queryResult, err := q.client.QueryWorkflow(ctx, order.OrderWorkflowID(orderID), "", order.ShippingStatusQuery)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return "", outbound.ErrOrderWorkflowNotFound
		}
		return "", fmt.Errorf("error querying shipping status: %w", err)
	}
	var status order.ShippingStatus
	if err = queryResult.Get(&status); err != nil {
		return "", fmt.Errorf("error reading shipping status: %w", err)
	}
	return string(status), nil
}
