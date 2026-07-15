package temporal_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/temporal"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"
)

var errBoom = errors.New("boom")

// fakeTemporalClient embeds client.Client so it satisfies the whole interface
// while implementing only ExecuteWorkflow. Any other method the adapter might
// call hits the nil embedded interface and panics, surfacing as a test failure.
type fakeTemporalClient struct {
	temporalclient.Client
	executeFn func(
		ctx context.Context,
		opts temporalclient.StartWorkflowOptions,
		workflow any,
		args ...any,
	) (temporalclient.WorkflowRun, error)
}

func (f fakeTemporalClient) ExecuteWorkflow(
	ctx context.Context,
	opts temporalclient.StartWorkflowOptions,
	workflow any,
	args ...any,
) (temporalclient.WorkflowRun, error) {
	return f.executeFn(ctx, opts, workflow, args...)
}

// fakeWorkflowRun stands in for a started run; Get copies results into valuePtr
// the way the real client does after the workflow completes.
type fakeWorkflowRun struct {
	temporalclient.WorkflowRun
	id      string
	results []pricesearch.QuoteResult
	getErr  error
}

func (f fakeWorkflowRun) GetID() string    { return f.id }
func (f fakeWorkflowRun) GetRunID() string { return "run-id" }
func (f fakeWorkflowRun) Get(_ context.Context, valuePtr any) error {
	if f.getErr != nil {
		return f.getErr
	}
	out, ok := valuePtr.(*[]pricesearch.QuoteResult)
	if !ok {
		return errors.New("unexpected valuePtr type")
	}
	*out = f.results
	return nil
}

// newSearcher builds the adapter around the given fake client.
func newSearcher(t *testing.T, c temporalclient.Client) *temporal.PriceSearcher {
	t.Helper()

	s, err := temporal.NewPriceSearcher(temporal.NewPriceSearcherParams{
		Client: c,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return s
}

// criteria builds valid search criteria for atorvastatin 20mg.
func criteria(t *testing.T) pharmacy.SearchCriteria {
	t.Helper()

	ms, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)
	c, err := pharmacy.NewSearchCriteria(pharmacy.NewSearchCriteriaParams{
		MedName:     "atorvastatin",
		MedStrength: ms,
	})
	require.NoError(t, err)
	return c
}

// runReturning builds a fake client whose workflow completes with the given results.
func runReturning(results []pricesearch.QuoteResult) fakeTemporalClient {
	return fakeTemporalClient{
		executeFn: func(
			context.Context, temporalclient.StartWorkflowOptions, any, ...any,
		) (temporalclient.WorkflowRun, error) {
			return fakeWorkflowRun{id: "price-search-1", results: results}, nil
		},
	}
}

// --- NewPriceSearcher -------------------------------------------------------

func TestNewPriceSearcher_Validation(t *testing.T) {
	tests := map[string]temporal.NewPriceSearcherParams{
		"missing logger": {Client: fakeTemporalClient{}},
		"missing client": {Logger: slog.New(slog.DiscardHandler)},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			s, err := temporal.NewPriceSearcher(params)
			require.Error(t, err)
			assert.Nil(t, s)
		})
	}
}

// --- Search: starting the workflow ------------------------------------------

func TestSearch_StartsWorkflowWithTranslatedInput(t *testing.T) {
	var gotOpts temporalclient.StartWorkflowOptions
	var gotWorkflow any
	var gotArgs []any

	c := fakeTemporalClient{
		executeFn: func(
			_ context.Context,
			opts temporalclient.StartWorkflowOptions,
			workflow any,
			args ...any,
		) (temporalclient.WorkflowRun, error) {
			gotOpts, gotWorkflow, gotArgs = opts, workflow, args
			return fakeWorkflowRun{id: opts.ID}, nil
		},
	}

	_, err := newSearcher(t, c).Search(context.Background(), criteria(t))
	require.NoError(t, err)

	// Routed at the workflow the worker registered, on its task queue.
	assert.Equal(t, pricesearch.WorkflowName, gotWorkflow)
	assert.Equal(t, pricesearch.TaskQueue, gotOpts.TaskQueue)
	// A run timeout bounds this synchronous call rather than blocking forever.
	assert.Positive(t, gotOpts.WorkflowRunTimeout)

	// Domain criteria are flattened into the DTO that crosses the Temporal boundary.
	require.Len(t, gotArgs, 1)
	in, ok := gotArgs[0].(pricesearch.SearchInput)
	require.True(t, ok, "workflow arg should be a SearchInput")
	assert.Equal(t, "atorvastatin", in.MedName)
	assert.Equal(t, "20", in.MedStrengthValue)
	assert.Equal(t, "mg", in.MedStrengthUnit)
}

func TestSearch_WorkflowIDIsUniquePerCall(t *testing.T) {
	// Each search is its own execution; a reused ID would collide with the
	// still-open run from the previous request.
	var ids []string
	c := fakeTemporalClient{
		executeFn: func(
			_ context.Context, opts temporalclient.StartWorkflowOptions, _ any, _ ...any,
		) (temporalclient.WorkflowRun, error) {
			ids = append(ids, opts.ID)
			return fakeWorkflowRun{id: opts.ID}, nil
		},
	}
	s := newSearcher(t, c)

	_, err := s.Search(context.Background(), criteria(t))
	require.NoError(t, err)
	_, err = s.Search(context.Background(), criteria(t))
	require.NoError(t, err)

	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1])
}

func TestSearch_ExecuteErrorSurfaces(t *testing.T) {
	c := fakeTemporalClient{
		executeFn: func(
			context.Context, temporalclient.StartWorkflowOptions, any, ...any,
		) (temporalclient.WorkflowRun, error) {
			return nil, errBoom
		},
	}

	_, err := newSearcher(t, c).Search(context.Background(), criteria(t))
	require.ErrorIs(t, err, errBoom)
}

func TestSearch_WorkflowFailureSurfaces(t *testing.T) {
	c := fakeTemporalClient{
		executeFn: func(
			context.Context, temporalclient.StartWorkflowOptions, any, ...any,
		) (temporalclient.WorkflowRun, error) {
			return fakeWorkflowRun{id: "price-search-1", getErr: errBoom}, nil
		},
	}

	_, err := newSearcher(t, c).Search(context.Background(), criteria(t))
	require.ErrorIs(t, err, errBoom)
}

// --- Search: mapping results back to the domain -----------------------------

func TestSearch_MapsResultsToPriceQuotes(t *testing.T) {
	pharmA, pharmB := uuid.New(), uuid.New()
	c := runReturning([]pricesearch.QuoteResult{
		{PharmacyID: pharmA, PharmacyItemID: "sku-1", UnitPriceCents: 880},
		{PharmacyID: pharmB, PharmacyItemID: "code-1", UnitPriceCents: 1161},
	})

	quotes, err := newSearcher(t, c).Search(context.Background(), criteria(t))
	require.NoError(t, err)
	require.Len(t, quotes, 2)

	// The workflow's ordering is preserved, and every DTO field lands on the VO.
	assert.Equal(t, pharmA, quotes[0].PharmacyID())
	assert.Equal(t, "sku-1", quotes[0].PharmacyItemID())
	assert.Equal(t, int64(880), quotes[0].Price().Cents())
	assert.Equal(t, pharmB, quotes[1].PharmacyID())
	assert.Equal(t, int64(1161), quotes[1].Price().Cents())
}

func TestSearch_NoResultsYieldsEmptySlice(t *testing.T) {
	quotes, err := newSearcher(t, runReturning(nil)).Search(context.Background(), criteria(t))
	require.NoError(t, err)
	assert.Empty(t, quotes)
}

func TestSearch_SkipsUnmappableResults(t *testing.T) {
	// A result the domain constructors reject is logged and dropped rather than
	// failing the whole search: the usable quotes still reach the caller.
	good := uuid.New()
	tests := map[string]pricesearch.QuoteResult{
		"negative price":      {PharmacyID: uuid.New(), PharmacyItemID: "sku-x", UnitPriceCents: -1},
		"missing pharmacy id": {PharmacyID: uuid.Nil, PharmacyItemID: "sku-x", UnitPriceCents: 500},
		"missing item id":     {PharmacyID: uuid.New(), PharmacyItemID: "", UnitPriceCents: 500},
	}
	for name, bad := range tests {
		t.Run(name, func(t *testing.T) {
			c := runReturning([]pricesearch.QuoteResult{
				bad,
				{PharmacyID: good, PharmacyItemID: "sku-1", UnitPriceCents: 880},
			})

			quotes, err := newSearcher(t, c).Search(context.Background(), criteria(t))
			require.NoError(t, err)
			require.Len(t, quotes, 1)
			assert.Equal(t, good, quotes[0].PharmacyID())
		})
	}
}
