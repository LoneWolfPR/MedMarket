package pricesearch

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/workflows/shared"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

// TaskQueue is the Temporal task queue the price-search worker polls and the
// starter targets in StartWorkflowOptions. Both sides must use this exact value.
const TaskQueue = "price-search"

// WorkflowName is the registered type name of PriceSearchWorkflow, shared by the
// worker's registration and the starter's ExecuteWorkflow. The Go function may be
// renamed; this value may not — versioning means a new name alongside the old.
const WorkflowName = "PriceSearchWorkflow"

// SearchInput contains the values needed to search from quotes from pharmacies
type SearchInput struct {
	MedName          string
	MedStrengthValue string
	MedStrengthUnit  string
}

// QuoteResult is a single quote from a pharmacy search
type QuoteResult struct {
	UnitPriceCents int64
	PharmacyID     uuid.UUID
	PharmacyItemID string
}

// PriceSearchWorkflow is the orchestrator for performing a search across pharmacies
// for prices of medications
//
//nolint:revive // Name of workflow is explicit for easy tracking in temporal
func PriceSearchWorkflow(ctx workflow.Context, i SearchInput) ([]QuoteResult, error) {
	var a *Activities
	ctx = shared.WithDefaultActivityOptions(ctx)
	logger := workflow.GetLogger(ctx)

	// Get a list of pharmacies in the system
	ctx = shared.WithScheduleToClose(ctx, 5*time.Second)
	ctx = shared.WithStartToCloseTimeout(ctx, 2*time.Second)
	var infos []PharmacyInfo
	listErr := workflow.ExecuteActivity(ctx, a.ListPharmaciesActivity).Get(ctx, &infos)
	if listErr != nil {
		return nil, fmt.Errorf("error fetching list of pharmacies: %w", listErr)
	}

	// Get a list of quotes from the pharmacies
	ctx = shared.WithScheduleToClose(ctx, 10*time.Second)
	ctx = shared.WithStartToCloseTimeout(ctx, 3*time.Second)
	results := []QuoteResult{}
	futures := make([]workflow.Future, 0, len(infos))
	for _, info := range infos {
		inputParams := SearchPharmacyInputParams{
			PharmInfo: info,
			Input:     i,
		}
		futures = append(futures, workflow.ExecuteActivity(ctx, a.SearchPharmacyActivity, inputParams))
	}
	for idx, future := range futures {
		var result []QuoteResult
		if searchErr := future.Get(ctx, &result); searchErr != nil {
			logger.Warn("error getting price quote", "pharmacy_code", infos[idx].Code, "error", searchErr)
			continue
		}
		results = append(results, result...)
	}
	slices.SortFunc(results, func(a, b QuoteResult) int {
		return cmp.Compare(a.UnitPriceCents, b.UnitPriceCents)
	})
	return results, nil
}
