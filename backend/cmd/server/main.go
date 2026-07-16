// Command server runs the MedMarket backend HTTP API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/bcrypt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/jwt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/s3"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/temporal"
	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/bootstrap"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	temporalclient "go.temporal.io/sdk/client"
)

const presignTTL = 15 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
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
	// Setup File Storage
	s3Client, err := newFileStorage(cfg, logger)
	if err != nil {
		return err
	}
	// Setup Prescription Repo
	rxRepo, err := postgres.NewPrescriptionRepository(postgres.NewPrescriptionRepositoryParams{
		Client: client,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("error creating prescription repo: %w", err)
	}
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

	// Setup Offer Repo
	offerRepo, err := postgres.NewOfferRepository(postgres.NewOfferRepositoryParams{
		Client: client,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("error creation offer repo: %w", err)
	}

	// Setup Temporal. The API only starts price-search workflows; the worker
	// executes them, so no worker/activity registration happens here.
	temporalClient, err := temporalclient.Dial(temporalclient.Options{
		HostPort:  cfg.TemporalHostPort,
		Namespace: "default",
	})
	if err != nil {
		return fmt.Errorf("error setting up temporal client: %w", err)
	}
	defer temporalClient.Close()

	priceSearcher, err := temporal.NewPriceSearcher(temporal.NewPriceSearcherParams{
		Client: temporalClient,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to set up price searcher: %w", err)
	}

	// Outbound adapters
	repo, err := postgres.NewUserRepository(postgres.NewUserRepositoryParams{
		Client: client,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to set up user repository: %w", err)
	}

	hasher, err := bcrypt.NewPasswordHasher(bcrypt.NewPasswordHasherParams{
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to set up password hasher: %w", err)
	}

	tokenIssuer, err := jwt.NewTokenIssuer(jwt.NewTokenIssuerParams{
		Logger: logger,
		Secret: cfg.JWTSecret,
		TTL:    cfg.JWTTTL,
	})
	if err != nil {
		return fmt.Errorf("failed to set up token issuer: %w", err)
	}

	// Application Services
	userService, err := app.NewUserService(app.NewUserServiceParams{
		Logger:         logger,
		UserRepository: repo,
		PasswordHasher: hasher,
		TokenIssuer:    tokenIssuer,
	})
	if err != nil {
		return fmt.Errorf("failed to set up user service: %w", err)
	}
	rxService, err := app.NewPrescriptionService(app.NewPrescriptionServiceParams{
		Logger:           logger,
		PrescriptionRepo: rxRepo,
		FileStorage:      s3Client,
	})
	if err != nil {
		return fmt.Errorf("failed to set up prescription service: %w", err)
	}
	priceSearchService, err := app.NewPriceSearchService(app.NewPriceSearchServiceParams{
		Logger:    logger,
		RxRepo:    rxRepo,
		PharmRepo: pharmacyRepo,
		Searcher:  priceSearcher,
		OfferRepo: offerRepo,
		OfferTTL:  cfg.OfferTTL,
	})
	if err != nil {
		return fmt.Errorf("failed to set up price search service: %w", err)
	}

	// Inbound Adapters
	authHandler, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{
		Logger: logger,
		Svc:    userService,
	})
	if err != nil {
		return fmt.Errorf("failed to setup auth handler: %w", err)
	}
	rxHandler, err := httpapi.NewPrescriptionHandler(httpapi.NewPrescriptionHandlerParams{
		Logger: logger,
		Svc:    rxService,
	})
	if err != nil {
		return fmt.Errorf("failed to setup prescription handler: %w", err)
	}
	priceSearchHandler, err := httpapi.NewPriceSearchHandler(httpapi.NewPriceSearchHandlerParams{
		Logger: logger,
		Svc:    priceSearchService,
	})
	if err != nil {
		return fmt.Errorf("failed to setup price search handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	handler := httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         authHandler,
		Prescription: rxHandler,
		Search:       priceSearchHandler,
		Logger:       logger,
		TokenIssuer:  tokenIssuer,
	}, mux)

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
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
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
