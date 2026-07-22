// Command server runs the MedMarket backend HTTP API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/bcrypt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/jwt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/s3"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/temporal"
	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/bootstrap"
	"github.com/LoneWolfPR/MedMarket/backend/internal/telemetry"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
)

const presignTTL = 15 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

	if err := bootstrap.SeedPharmacies(context.Background(), pharmacyRepo); err != nil {
		return fmt.Errorf("error seeding pharmacy data: %w", err)
	}

	// Setup Open Telemetry
	otelShutdown, err := telemetry.Setup(ctx, cfg.OTLPEndpoint, "medmarket-backend")
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

	ti, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{})
	if err != nil {
		return fmt.Errorf("error setting up tracing interceptor: %w", err)
	}

	// Setup Temporal. The API only starts price-search workflows; the worker
	// executes them, so no worker/activity registration happens here.
	temporalClient, err := temporalclient.Dial(temporalclient.Options{
		HostPort:     cfg.TemporalHostPort,
		Namespace:    "default",
		Interceptors: []interceptor.ClientInterceptor{ti},
	})
	if err != nil {
		return fmt.Errorf("error setting up temporal client: %w", err)
	}
	defer temporalClient.Close()

	handler, err := buildHandler(buildHandlerParams{
		Logger:         logger,
		Config:         cfg,
		EntClient:      client,
		TemporalClient: temporalClient,
	})
	if err != nil {
		return fmt.Errorf("error building handler: %w", err)
	}

	// Explicit timeouts guard against slow-client attacks (e.g. Slowloris);
	// the bare http.ListenAndServe uses a zero-value server with none, which
	// is what gosec flags (G114).
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("backend listening", "addr", srv.Addr)
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		return err
	case <-ctx.Done():
		shutDownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutDownCtx)
	}
}

type buildHandlerParams struct {
	Logger         *slog.Logger
	Config         config
	EntClient      *ent.Client
	TemporalClient temporalclient.Client
}

func buildHandler(p buildHandlerParams) (http.Handler, error) {
	// Setup File Storage
	s3Client, err := newFileStorage(p.Config, p.Logger)
	if err != nil {
		return nil, err
	}
	// Setup Prescription Repo
	rxRepo, err := postgres.NewPrescriptionRepository(postgres.NewPrescriptionRepositoryParams{
		Client: p.EntClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating prescription repo: %w", err)
	}

	// Pharmacy Repo
	pharmacyRepo, err := postgres.NewPharmacyRepository(postgres.NewPharmacyRepositoryParams{
		Client: p.EntClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating pharmacy repo: %w", err)
	}

	// Setup Offer Repo
	offerRepo, err := postgres.NewOfferRepository(postgres.NewOfferRepositoryParams{
		Client: p.EntClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating offer repo: %w", err)
	}

	// Setup Order Repo
	orderRepo, err := postgres.NewOrderRepository(postgres.NewOrderRepositoryParams{
		Client: p.EntClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating order repo: %w", err)
	}

	priceSearcher, err := temporal.NewPriceSearcher(temporal.NewPriceSearcherParams{
		Client: p.TemporalClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up price searcher: %w", err)
	}

	orderStarter, err := temporal.NewOrderStarter(temporal.NewOrderStarterParams{
		Client:         p.TemporalClient,
		Logger:         p.Logger,
		WebhookBaseURL: p.Config.WebhookBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup order starter: %w", err)
	}

	shippingSignaler, err := temporal.NewShippingSignaler(temporal.NewShippingSignalerParams{
		Client: p.TemporalClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup shipping signaler: %w", err)
	}

	orderStatusQuerier, err := temporal.NewOrderStatusQuerier(temporal.NewOrderStatusQuerierParams{
		Client: p.TemporalClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up order status querier: %w", err)
	}

	// Outbound adapters
	userRepo, err := postgres.NewUserRepository(postgres.NewUserRepositoryParams{
		Client: p.EntClient,
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up user repository: %w", err)
	}

	hasher, err := bcrypt.NewPasswordHasher(bcrypt.NewPasswordHasherParams{
		Logger: p.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up password hasher: %w", err)
	}

	tokenIssuer, err := jwt.NewTokenIssuer(jwt.NewTokenIssuerParams{
		Logger: p.Logger,
		Secret: p.Config.JWTSecret,
		TTL:    p.Config.JWTTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up token issuer: %w", err)
	}

	// Application Services
	userService, err := app.NewUserService(app.NewUserServiceParams{
		Logger:         p.Logger,
		UserRepository: userRepo,
		PasswordHasher: hasher,
		TokenIssuer:    tokenIssuer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up user service: %w", err)
	}
	rxService, err := app.NewPrescriptionService(app.NewPrescriptionServiceParams{
		Logger:           p.Logger,
		PrescriptionRepo: rxRepo,
		FileStorage:      s3Client,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up prescription service: %w", err)
	}
	priceSearchService, err := app.NewPriceSearchService(app.NewPriceSearchServiceParams{
		Logger:    p.Logger,
		RxRepo:    rxRepo,
		PharmRepo: pharmacyRepo,
		Searcher:  priceSearcher,
		OfferRepo: offerRepo,
		OfferTTL:  p.Config.OfferTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up price search service: %w", err)
	}

	orderService, err := app.NewOrderService(app.NewOrderServiceParams{
		Logger:       p.Logger,
		OrderRepo:    orderRepo,
		OfferRepo:    offerRepo,
		PharmRepo:    pharmacyRepo,
		RxRepo:       rxRepo,
		UserRepo:     userRepo,
		OrderStarter: orderStarter,
		Querier:      orderStatusQuerier,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up order service: %w", err)
	}

	shippingService, err := app.NewShippingService(app.NewShippingServiceParams{
		Logger:   p.Logger,
		Signaler: shippingSignaler,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up shipping signaler: %w", err)
	}

	// Inbound Adapters
	authHandler, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{
		Logger: p.Logger,
		Svc:    userService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup auth handler: %w", err)
	}
	rxHandler, err := httpapi.NewPrescriptionHandler(httpapi.NewPrescriptionHandlerParams{
		Logger: p.Logger,
		Svc:    rxService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup prescription handler: %w", err)
	}
	priceSearchHandler, err := httpapi.NewPriceSearchHandler(httpapi.NewPriceSearchHandlerParams{
		Logger: p.Logger,
		Svc:    priceSearchService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup price search handler: %w", err)
	}
	orderHandler, err := httpapi.NewOrderHandler(httpapi.NewOrderHandlerParams{
		Logger: p.Logger,
		Svc:    orderService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup order handler: %w", err)
	}
	shippingHandler, err := httpapi.NewShippingHandler(httpapi.NewShippingHandlerParams{
		Logger: p.Logger,
		Svc:    shippingService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up shipping handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("GET /api/openapi.yaml", openAPISpecHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	handler := httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         authHandler,
		Prescription: rxHandler,
		Search:       priceSearchHandler,
		Logger:       p.Logger,
		TokenIssuer:  tokenIssuer,
		Order:        orderHandler,
		Shipping:     shippingHandler,
	}, mux)
	handlerWithOtel := otelhttp.NewHandler(handler, "http.server")
	return handlerWithOtel, nil
}

// newFileStorage builds the S3/MinIO-backed file storage adapter. It uses two
// clients: an internal one for uploads and a public-endpoint one for presigned
// URLs the browser can reach (see the split-horizon note in the s3 package).
func newFileStorage(cfg config, logger *slog.Logger) (*s3.S3, error) {
	minioClient, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting up minio client: %w", err)
	}
	minioPublicClient, err := minio.New(cfg.MinioPublicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Region: cfg.MinioRegion,
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting up public minio client: %w", err)
	}
	return s3.NewS3(s3.NewS3Params{
		Client:        minioClient,
		PresignClient: minioPublicClient,
		Bucket:        cfg.MinioBucket,
		PresignTTL:    presignTTL,
		Logger:        logger,
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}
