package pricesearch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"
)

// newWorkflowEnv builds a Temporal test environment with the activity struct
// registered so the workflow's activity references resolve by name. Every test
// mocks the activities with OnActivity, so the real implementations never run.
func newWorkflowEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(&pricesearch.Activities{})
	t.Cleanup(func() { env.AssertExpectations(t) })
	return env
}

// executeSearch runs the workflow to completion and returns its results.
func executeSearch(t *testing.T, env *testsuite.TestWorkflowEnvironment) []pricesearch.QuoteResult {
	t.Helper()

	env.ExecuteWorkflow(pricesearch.PriceSearchWorkflow, pricesearch.SearchInput{
		MedName:          "atorvastatin",
		MedStrengthValue: "20",
		MedStrengthUnit:  "mg",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var results []pricesearch.QuoteResult
	require.NoError(t, env.GetWorkflowResult(&results))
	return results
}

// pharmacyInfo builds a PharmacyInfo the list activity can return.
func pharmacyInfo(id uuid.UUID, code, name string) pricesearch.PharmacyInfo {
	return pricesearch.PharmacyInfo{ID: id, Code: code, Name: name}
}

func TestPriceSearchWorkflow_ListFailureFailsWorkflow(t *testing.T) {
	// Without the pharmacy list there is nothing to fan out to, so this is fatal
	// rather than a partial result. No call-count constraint: the shared retry
	// policy retries the failing activity until ScheduleToClose expires.
	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).Return(nil, errBoom)

	env.ExecuteWorkflow(pricesearch.PriceSearchWorkflow, pricesearch.SearchInput{
		MedName: "atorvastatin", MedStrengthValue: "20", MedStrengthUnit: "mg",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
}

func TestPriceSearchWorkflow_NoPharmaciesYieldsNoQuotes(t *testing.T) {
	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).
		Return([]pricesearch.PharmacyInfo{}, nil).Once()
	// No pharmacies means the search activity is never scheduled; AssertExpectations
	// would fail if the workflow called it anyway.

	results := executeSearch(t, env)
	assert.Empty(t, results)
}

func TestPriceSearchWorkflow_FansOutToEveryPharmacy(t *testing.T) {
	idA, idB := uuid.New(), uuid.New()
	infos := []pricesearch.PharmacyInfo{
		pharmacyInfo(idA, "mock-pharmacy-a", "Mock Pharmacy A"),
		pharmacyInfo(idB, "mock-pharmacy-b", "Mock Pharmacy B"),
	}

	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).Return(infos, nil).Once()

	// Each pharmacy gets exactly one search, carrying that pharmacy's info and the
	// workflow's original input.
	var gotParams []pricesearch.SearchPharmacyInputParams
	env.OnActivity(a.SearchPharmacyActivity, mock.Anything, mock.Anything).
		Return(func(
			_ context.Context, p pricesearch.SearchPharmacyInputParams,
		) ([]pricesearch.QuoteResult, error) {
			gotParams = append(gotParams, p)
			return []pricesearch.QuoteResult{
				{PharmacyID: p.PharmInfo.ID, PharmacyItemID: "sku", UnitPriceCents: 500},
			}, nil
		}).Twice()

	results := executeSearch(t, env)
	require.Len(t, results, 2)

	require.Len(t, gotParams, 2)
	for _, p := range gotParams {
		assert.Equal(t, "atorvastatin", p.Input.MedName)
		assert.Equal(t, "20", p.Input.MedStrengthValue)
		assert.Equal(t, "mg", p.Input.MedStrengthUnit)
	}
	assert.ElementsMatch(t,
		[]uuid.UUID{idA, idB},
		[]uuid.UUID{gotParams[0].PharmInfo.ID, gotParams[1].PharmInfo.ID},
	)
}

func TestPriceSearchWorkflow_SortsQuotesByPriceAscending(t *testing.T) {
	idA, idB := uuid.New(), uuid.New()
	infos := []pricesearch.PharmacyInfo{
		pharmacyInfo(idA, "mock-pharmacy-a", "Mock Pharmacy A"),
		pharmacyInfo(idB, "mock-pharmacy-b", "Mock Pharmacy B"),
	}

	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).Return(infos, nil).Once()

	// The first pharmacy answers with the pricier quotes, so ordering can only be
	// correct if the workflow sorts rather than relying on fan-out order.
	env.OnActivity(a.SearchPharmacyActivity, mock.Anything, mock.Anything).
		Return(func(
			_ context.Context, p pricesearch.SearchPharmacyInputParams,
		) ([]pricesearch.QuoteResult, error) {
			if p.PharmInfo.ID == idA {
				return []pricesearch.QuoteResult{
					{PharmacyID: idA, PharmacyItemID: "sku-pricy", UnitPriceCents: 1500},
					{PharmacyID: idA, PharmacyItemID: "sku-mid", UnitPriceCents: 1000},
				}, nil
			}
			return []pricesearch.QuoteResult{
				{PharmacyID: idB, PharmacyItemID: "code-cheap", UnitPriceCents: 300},
			}, nil
		}).Twice()

	results := executeSearch(t, env)
	require.Len(t, results, 3)

	prices := []int64{results[0].UnitPriceCents, results[1].UnitPriceCents, results[2].UnitPriceCents}
	assert.Equal(t, []int64{300, 1000, 1500}, prices, "quotes should be cheapest first")
}

func TestPriceSearchWorkflow_OneSearchFailureIsSkipped(t *testing.T) {
	// One pharmacy being down must not sink the whole search: the survivors' quotes
	// are still returned.
	idA, idB := uuid.New(), uuid.New()
	infos := []pricesearch.PharmacyInfo{
		pharmacyInfo(idA, "mock-pharmacy-a", "Mock Pharmacy A"),
		pharmacyInfo(idB, "mock-pharmacy-b", "Mock Pharmacy B"),
	}

	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).Return(infos, nil).Once()
	env.OnActivity(a.SearchPharmacyActivity, mock.Anything, mock.Anything).
		Return(func(
			_ context.Context, p pricesearch.SearchPharmacyInputParams,
		) ([]pricesearch.QuoteResult, error) {
			if p.PharmInfo.ID == idA {
				return nil, errBoom
			}
			return []pricesearch.QuoteResult{
				{PharmacyID: idB, PharmacyItemID: "code-1", UnitPriceCents: 880},
			}, nil
		})

	results := executeSearch(t, env)
	require.Len(t, results, 1)
	assert.Equal(t, idB, results[0].PharmacyID)
}

func TestPriceSearchWorkflow_AllSearchesFailYieldNoQuotes(t *testing.T) {
	// Every pharmacy failing is an empty result, not a workflow error — the caller
	// gets "no quotes" rather than a 500.
	infos := []pricesearch.PharmacyInfo{
		pharmacyInfo(uuid.New(), "mock-pharmacy-a", "Mock Pharmacy A"),
		pharmacyInfo(uuid.New(), "mock-pharmacy-b", "Mock Pharmacy B"),
	}

	var a *pricesearch.Activities
	env := newWorkflowEnv(t)
	env.OnActivity(a.ListPharmaciesActivity, mock.Anything).Return(infos, nil).Once()
	env.OnActivity(a.SearchPharmacyActivity, mock.Anything, mock.Anything).
		Return(nil, errBoom)

	results := executeSearch(t, env)
	assert.Empty(t, results)
}
