package temporal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

const workflowTimeOut = 30 * time.Second

// PriceSearcher is the adapter for running a price search temporal
// workflow
type PriceSearcher struct {
	client client.Client
	logger *slog.Logger
}

var _ outbound.PriceSearcher = (*PriceSearcher)(nil)

// NewPriceSearcherParams contains the input params necessary to
// construct a new instance
type NewPriceSearcherParams struct {
	Client client.Client
	Logger *slog.Logger
}

// NewPriceSearcher constructs a new instance
func NewPriceSearcher(p NewPriceSearcherParams) (*PriceSearcher, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Client == nil {
		return nil, errors.New("temporal client is missing")
	}
	return &PriceSearcher{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Search runs a temporal workflow to get a list of prices for a med
func (s *PriceSearcher) Search(
	ctx context.Context,
	criteria pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	workflowOpts := client.StartWorkflowOptions{
		ID:                 fmt.Sprintf("price-search-%s", uuid.New()),
		TaskQueue:          pricesearch.TaskQueue,
		WorkflowRunTimeout: workflowTimeOut,
	}

	input := pricesearch.SearchInput{
		MedName:          criteria.MedName(),
		MedStrengthValue: criteria.MedStrength().Value(),
		MedStrengthUnit:  criteria.MedStrength().Unit(),
	}

	run, err := s.client.ExecuteWorkflow(ctx, workflowOpts, pricesearch.WorkflowName, input)
	if err != nil {
		return nil, fmt.Errorf("error starting workflow: %w", err)
	}

	var workflowResults []pricesearch.QuoteResult
	if err = run.Get(ctx, &workflowResults); err != nil {
		return nil, fmt.Errorf("workflow %s execution failed: %w", run.GetID(), err)
	}

	resp := []pharmacy.PriceQuote{}
	for _, result := range workflowResults {
		newPrice, err := shared.NewMoneyFromCents(result.UnitPriceCents)
		if err != nil {
			s.logger.ErrorContext(ctx,
				"error reading price on quote",
				"unit price cents",
				result.UnitPriceCents,
				"pharmacy id",
				result.PharmacyID,
				"pharmacy item id",
				result.PharmacyItemID,
				"error",
				err,
			)
			continue
		}
		quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
			Price:          newPrice,
			PharmacyID:     result.PharmacyID,
			PharmacyItemID: result.PharmacyItemID,
		})
		if err != nil {
			s.logger.ErrorContext(ctx,
				"error mapping to PriceQuote",
				"unit price cents",
				result.UnitPriceCents,
				"pharmacy id",
				result.PharmacyID,
				"pharmacy item id",
				result.PharmacyItemID,
				"error",
				err,
			)
			continue
		}
		resp = append(resp, quote)
	}
	s.logger.InfoContext(
		ctx,
		"workflow completed",
		"workflow id",
		run.GetID(),
		"run id",
		run.GetRunID(),
		"number of results",
		len(resp),
	)
	return resp, nil
}
