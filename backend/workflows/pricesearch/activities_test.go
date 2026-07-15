package pricesearch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"
)

// validSearchInput returns a SearchInput whose values rebuild into valid domain
// objects inside the activity.
func validSearchInput() pricesearch.SearchInput {
	return pricesearch.SearchInput{
		MedName:          "atorvastatin",
		MedStrengthValue: "20",
		MedStrengthUnit:  "mg",
	}
}

// isNonRetryable reports whether err is an ApplicationError Temporal will not retry.
func isNonRetryable(t *testing.T, err error) bool {
	t.Helper()

	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr, "expected a temporal ApplicationError")
	return appErr.NonRetryable()
}

// --- ListPharmaciesActivity -------------------------------------------------

func TestListPharmaciesActivity_RepoError(t *testing.T) {
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, errBoom }},
		Clients: map[string]outbound.PharmacyClient{},
	})

	_, err := a.ListPharmaciesActivity(context.Background())
	require.ErrorIs(t, err, errBoom)
}

func TestListPharmaciesActivity_MapsRegisteredPharmacies(t *testing.T) {
	idA, idB := uuid.New(), uuid.New()
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo: fakePharmacyRepo{
			listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
				return []pharmacy.Pharmacy{
					buildPharmacy(t, idA, "mock-pharmacy-a", "Mock Pharmacy A"),
					buildPharmacy(t, idB, "mock-pharmacy-b", "Mock Pharmacy B"),
				}, nil
			},
		},
		Clients: map[string]outbound.PharmacyClient{
			"mock-pharmacy-a": fakePharmacyClient{},
			"mock-pharmacy-b": fakePharmacyClient{},
		},
	})

	infos, err := a.ListPharmaciesActivity(context.Background())
	require.NoError(t, err)
	require.Len(t, infos, 2)
	assert.Equal(t, pricesearch.PharmacyInfo{ID: idA, Code: "mock-pharmacy-a", Name: "Mock Pharmacy A"}, infos[0])
	assert.Equal(t, pricesearch.PharmacyInfo{ID: idB, Code: "mock-pharmacy-b", Name: "Mock Pharmacy B"}, infos[1])
}

func TestListPharmaciesActivity_SkipsPharmaciesWithoutAClient(t *testing.T) {
	// The DB is the source of truth for pharmacies, but only those this worker has
	// an adapter registered for can actually be searched. A row with no client (a
	// pharmacy added to the DB but not yet wired up) is dropped, not an error.
	wired := uuid.New()
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo: fakePharmacyRepo{
			listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
				return []pharmacy.Pharmacy{
					buildPharmacy(t, wired, "mock-pharmacy-a", "Mock Pharmacy A"),
					buildPharmacy(t, uuid.New(), "unwired-pharmacy", "Unwired Pharmacy"),
				}, nil
			},
		},
		Clients: map[string]outbound.PharmacyClient{
			"mock-pharmacy-a": fakePharmacyClient{},
		},
	})

	infos, err := a.ListPharmaciesActivity(context.Background())
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, wired, infos[0].ID)
}

func TestListPharmaciesActivity_NoPharmaciesYieldsEmptySlice(t *testing.T) {
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, nil }},
		Clients: map[string]outbound.PharmacyClient{},
	})

	infos, err := a.ListPharmaciesActivity(context.Background())
	require.NoError(t, err)
	assert.Empty(t, infos)
}

// --- SearchPharmacyActivity -------------------------------------------------

func TestSearchPharmacyActivity_UnknownClientIsNonRetryable(t *testing.T) {
	// No adapter for this code means no retry will ever help.
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{},
		Clients: map[string]outbound.PharmacyClient{},
	})

	_, err := a.SearchPharmacyActivity(context.Background(), pricesearch.SearchPharmacyInputParams{
		PharmInfo: pricesearch.PharmacyInfo{ID: uuid.New(), Code: "nope", Name: "Nope"},
		Input:     validSearchInput(),
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err), "a missing client must not be retried")
}

func TestSearchPharmacyActivity_InvalidInputIsNonRetryable(t *testing.T) {
	// The DTO carries raw strings across the Temporal boundary; rebuilding the
	// value objects is where bad input is caught. It is terminal for this input.
	tests := map[string]pricesearch.SearchInput{
		"bad strength unit":  {MedName: "atorvastatin", MedStrengthValue: "20", MedStrengthUnit: "invalid"},
		"bad strength value": {MedName: "atorvastatin", MedStrengthValue: "zero", MedStrengthUnit: "mg"},
		"empty med name":     {MedName: "", MedStrengthValue: "20", MedStrengthUnit: "mg"},
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			client := fakePharmacyClient{
				searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
					t.Fatal("the pharmacy must not be called with invalid input")
					return nil, nil
				},
			}
			a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
				Repo:    fakePharmacyRepo{},
				Clients: map[string]outbound.PharmacyClient{"mock-pharmacy-a": client},
			})

			_, err := a.SearchPharmacyActivity(context.Background(), pricesearch.SearchPharmacyInputParams{
				PharmInfo: pricesearch.PharmacyInfo{ID: uuid.New(), Code: "mock-pharmacy-a", Name: "A"},
				Input:     in,
			})
			require.Error(t, err)
			assert.True(t, isNonRetryable(t, err), "invalid input must not be retried")
		})
	}
}

func TestSearchPharmacyActivity_ClientErrorIsRetryable(t *testing.T) {
	// A pharmacy being down is transient, so this error must stay retryable and
	// let the retry policy park-and-heal.
	client := fakePharmacyClient{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return nil, errBoom
		},
	}
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{},
		Clients: map[string]outbound.PharmacyClient{"mock-pharmacy-a": client},
	})

	_, err := a.SearchPharmacyActivity(context.Background(), pricesearch.SearchPharmacyInputParams{
		PharmInfo: pricesearch.PharmacyInfo{ID: uuid.New(), Code: "mock-pharmacy-a", Name: "A"},
		Input:     validSearchInput(),
	})
	require.ErrorIs(t, err, errBoom)

	// It must not be marked non-retryable; a plain wrapped error is retryable by
	// default, which is what the park-and-heal retry policy expects.
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		assert.False(t, appErr.NonRetryable(), "a pharmacy failure must remain retryable")
	}
}

func TestSearchPharmacyActivity_BuildsCriteriaAndMapsQuotes(t *testing.T) {
	pharmID := uuid.New()

	var got pharmacy.SearchCriteria
	client := fakePharmacyClient{
		searchFn: func(_ context.Context, c pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			got = c
			return []pharmacy.PriceQuote{
				buildQuote(t, pharmID, "sku-1", 880),
				buildQuote(t, pharmID, "sku-2", 1200),
			}, nil
		},
	}
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{},
		Clients: map[string]outbound.PharmacyClient{"mock-pharmacy-a": client},
	})

	results, err := a.SearchPharmacyActivity(context.Background(), pricesearch.SearchPharmacyInputParams{
		PharmInfo: pricesearch.PharmacyInfo{ID: pharmID, Code: "mock-pharmacy-a", Name: "A"},
		Input:     validSearchInput(),
	})
	require.NoError(t, err)

	// The DTO's raw strings were rebuilt into domain criteria for the port.
	assert.Equal(t, "atorvastatin", got.MedName())
	assert.Equal(t, "20", got.MedStrength().Value())
	assert.Equal(t, "mg", got.MedStrength().Unit())

	// Domain quotes were flattened back into DTOs for the workflow.
	require.Len(t, results, 2)
	assert.Equal(t, pricesearch.QuoteResult{
		UnitPriceCents: 880, PharmacyID: pharmID, PharmacyItemID: "sku-1",
	}, results[0])
	assert.Equal(t, pricesearch.QuoteResult{
		UnitPriceCents: 1200, PharmacyID: pharmID, PharmacyItemID: "sku-2",
	}, results[1])
}

func TestSearchPharmacyActivity_NoQuotesYieldsEmptySlice(t *testing.T) {
	// A pharmacy not stocking the medication is a normal outcome, not an error.
	client := fakePharmacyClient{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return nil, nil
		},
	}
	a := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    fakePharmacyRepo{},
		Clients: map[string]outbound.PharmacyClient{"mock-pharmacy-a": client},
	})

	results, err := a.SearchPharmacyActivity(context.Background(), pricesearch.SearchPharmacyInputParams{
		PharmInfo: pricesearch.PharmacyInfo{ID: uuid.New(), Code: "mock-pharmacy-a", Name: "A"},
		Input:     validSearchInput(),
	})
	require.NoError(t, err)
	assert.Empty(t, results)
}
