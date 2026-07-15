// Command worker runs the MedMarket temporal worker
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacya"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacyb"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/bootstrap"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Env Config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup DB
	client, err := bootstrap.NewEntClient(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			logger.Error("failed to close db client", "error", cerr)
		}
	}()

	// Setup seeding of pharmacy data
	pharmacyRepo, err := postgres.NewPharmacyRepository(postgres.NewPharmacyRepositoryParams{
		Client: client,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("error creating pharmacy repo: %w", err)
	}

	if err := bootstrap.SeedPharmacies(ctx, pharmacyRepo); err != nil {
		return fmt.Errorf("error seeding pharmacy data: %w", err)
	}

	// Setup pharmacy adapters
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	pharmA, err := pharmacyRepo.GetByCode(ctx, bootstrap.PharmacyACode)
	if err != nil {
		return fmt.Errorf("error fetching pharmacy with code %s: %w", bootstrap.PharmacyACode, err)
	}
	pharmAAdapter, err := pharmacya.NewPharmacyA(pharmacya.NewPharmacyAParams{
		Client:  httpClient,
		Logger:  logger,
		ID:      pharmA.ID,
		Secret:  cfg.PharmacyASecret,
		BaseURL: cfg.PharmacyABaseURL,
	})
	if err != nil {
		return fmt.Errorf("error creating adapter for pharmacy with code %s: %w", pharmA.Code, err)
	}
	pharmB, err := pharmacyRepo.GetByCode(ctx, bootstrap.PharmacyBCode)
	if err != nil {
		return fmt.Errorf("error fetching pharmacy with code %s: %w", bootstrap.PharmacyBCode, err)
	}
	pharmBAdapter, err := pharmacyb.NewPharmacyB(pharmacyb.NewPharmacyBParams{
		Client:  httpClient,
		Logger:  logger,
		ID:      pharmB.ID,
		Secret:  cfg.PharmacyBSecret,
		BaseURL: cfg.PharmacyBBaseURL,
	})
	if err != nil {
		return fmt.Errorf("error creating adapter for pharmacy with code %s: %w", pharmB.Code, err)
	}

	pharmClients := map[string]outbound.PharmacyClient{
		bootstrap.PharmacyACode: pharmAAdapter,
		bootstrap.PharmacyBCode: pharmBAdapter,
	}

	// Set up Temporal
	activities := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    pharmacyRepo,
		Clients: pharmClients,
	})
	c, err := temporalclient.Dial(temporalclient.Options{
		HostPort:  cfg.TemporalHostPort,
		Namespace: "default",
	})
	if err != nil {
		return fmt.Errorf("error setting up temporal client: %w", err)
	}
	defer c.Close()

	w := worker.New(c, pricesearch.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(pricesearch.PriceSearchWorkflow, workflow.RegisterOptions{
		Name: pricesearch.WorkflowName,
	})
	w.RegisterActivity(activities)
	return w.Run(worker.InterruptCh())
}
