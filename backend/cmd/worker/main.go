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

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/mailer"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacya"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacyb"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/shipping"
	internalstripe "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/stripe"
	"github.com/LoneWolfPR/MedMarket/backend/internal/bootstrap"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/telemetry"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/order"
	"github.com/LoneWolfPR/MedMarket/backend/workflows/pricesearch"

	"github.com/stripe/stripe-go/v86"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
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

	// Setup Open Telemetry
	otelShutdown, err := telemetry.Setup(ctx, cfg.OTLPEndpoint, "medmarket-worker")
	if err != nil {
		return fmt.Errorf("error setting up open telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down telemetry", "error", err)
		}
	}()

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
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
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

	orderRepo, err := postgres.NewOrderRepository(postgres.NewOrderRepositoryParams{
		Client: client,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("error creating order repository: %w", err)
	}

	// Set up other adapters
	shippingClient, err := shipping.NewShippingClient(shipping.NewShippingClientParams{
		Client:  httpClient,
		Logger:  logger,
		BaseURL: cfg.ShippingBaseURL,
	})
	if err != nil {
		return fmt.Errorf("error setting up shipping client: %w", err)
	}

	mailerAdapter, err := mailer.NewMailer(mailer.NewMailerParams{
		Logger: logger,
		Auth:   nil,
		Addr:   cfg.SMTPHost + ":" + cfg.SMTPPort,
		From:   cfg.SMTPFrom,
	})
	if err != nil {
		return fmt.Errorf("error setting up mailer: %w", err)
	}

	stripeClient := stripe.NewClient(cfg.StripeSecretKey)
	paymentGateway, err := internalstripe.NewPaymentGateway(internalstripe.NewPaymentGatewayParams{
		Client: stripeClient,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("error setting up payment gateway: %w", err)
	}

	// Set up Temporal
	priceSearchActivities := pricesearch.NewActivities(pricesearch.NewActivitiesParams{
		Repo:    pharmacyRepo,
		Clients: pharmClients,
	})
	orderActivities := order.NewActivities(order.NewActivitiesParams{
		Clients:        pharmClients,
		OrderRepo:      orderRepo,
		ShippingClient: shippingClient,
		EmailSender:    mailerAdapter,
		PaymentGateway: paymentGateway,
	})

	ti, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{})
	if err != nil {
		return fmt.Errorf("error setting up tracing interceptor: %w", err)
	}

	c, err := temporalclient.Dial(temporalclient.Options{
		HostPort:     cfg.TemporalHostPort,
		Namespace:    "default",
		Interceptors: []interceptor.ClientInterceptor{ti},
	})
	if err != nil {
		return fmt.Errorf("error setting up temporal client: %w", err)
	}
	defer c.Close()

	priceSearchWorker := worker.New(c, pricesearch.TaskQueue, worker.Options{})
	priceSearchWorker.RegisterWorkflowWithOptions(pricesearch.PriceSearchWorkflow, workflow.RegisterOptions{
		Name: pricesearch.WorkflowName,
	})
	priceSearchWorker.RegisterActivity(priceSearchActivities)

	orderWorker := worker.New(c, order.TaskQueue, worker.Options{})
	orderWorker.RegisterWorkflowWithOptions(order.OrderWorkflow, workflow.RegisterOptions{
		Name: order.WorkflowName,
	})
	orderWorker.RegisterActivity(orderActivities)

	err = priceSearchWorker.Start()
	if err != nil {
		return fmt.Errorf("error starting price search worker: %w", err)
	}
	defer priceSearchWorker.Stop()

	return orderWorker.Run(worker.InterruptCh())
}
