package temporal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	temporalorder "github.com/LoneWolfPR/MedMarket/backend/workflows/order"

	"go.temporal.io/sdk/client"
)

// OrderStarter is the adapter for running a order workflow
type OrderStarter struct {
	client         client.Client
	logger         *slog.Logger
	webhookBaseURL string
}

var _ outbound.OrderStarter = (*OrderStarter)(nil)

// NewOrderStarterParams holds the parameters necessary to construct an instance
type NewOrderStarterParams struct {
	Client         client.Client
	Logger         *slog.Logger
	WebhookBaseURL string
}

// NewOrderStarter constructs an instance of the adapter
func NewOrderStarter(p NewOrderStarterParams) (*OrderStarter, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.WebhookBaseURL == "" {
		return nil, errors.New("webhook base url is missing")
	}
	return &OrderStarter{
		client:         p.Client,
		logger:         p.Logger,
		webhookBaseURL: p.WebhookBaseURL,
	}, nil
}

// StartOrder launches the order tracking workflow
func (s *OrderStarter) StartOrder(ctx context.Context, i order.PlacementRequest) error {
	workflowID := temporalorder.OrderWorkflowID(i.OrderID())
	workflowOpts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: temporalorder.TaskQueue,
	}

	shippingAddress := temporalorder.ShippingAddress{
		Street1: i.Address().Street1,
		Street2: i.Address().Street2,
		City:    i.Address().City,
		State:   i.Address().State,
		Zip:     i.Address().Zip,
	}

	input := temporalorder.Input{
		IdempotencyKey: workflowID,
		OrderID:        i.OrderID(),
		PharmacyCode:   i.PharmacyCode(),
		PharmacyItemID: i.PharmacyItemID(),
		ItemName:       i.ItemName(),
		RecipientName:  i.RecipientName(),
		RecipientEmail: i.RecipientEmail(),
		Qty:            i.Qty(),
		Address:        shippingAddress,
		AmountCents:    i.AmountCents(),
		PaymentMethod:  i.PaymentMethod(),
		WebhookBaseURL: s.webhookBaseURL,
	}

	run, err := s.client.ExecuteWorkflow(ctx, workflowOpts, temporalorder.WorkflowName, input)
	if err != nil {
		return fmt.Errorf("error starting order workflow: %w", err)
	}

	s.logger.InfoContext(
		ctx,
		"order workflow started",
		"workflow_id",
		run.GetID(),
		"run_id",
		run.GetRunID(),
	)

	return nil
}
