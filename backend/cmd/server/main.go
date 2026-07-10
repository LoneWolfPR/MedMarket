// Command server runs the MedMarket backend HTTP API.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/bcrypt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/jwt"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/app"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type config struct {
	DatabaseURL      string
	JWTSecret        []byte
	JWTTTL           time.Duration
	Port             string
	PharmacyABaseURL string
	PharmacyASecret  string
	PharmacyBBaseURL string
	PharmacyBSecret  string
}

func loadConfig() (config, error) {
	dbURL := os.Getenv(EnvDBURL)
	if dbURL == "" {
		return config{}, fmt.Errorf("%s is missing", EnvDBURL)
	}
	jwtSecret := os.Getenv(EnvJWTSecret)
	if jwtSecret == "" {
		return config{}, fmt.Errorf("%s is missing", EnvJWTSecret)
	}
	var (
		jwtTTL time.Duration
		err    error
	)
	jwtTTLString := os.Getenv(EnvJWTTTL)
	if jwtTTLString == "" {
		jwtTTL = time.Hour * 24
	} else {
		jwtTTL, err = time.ParseDuration(jwtTTLString)
		if err != nil {
			return config{}, fmt.Errorf("error parsing jwt ttl: %w", err)
		}
	}
	port := "8080"
	if p := os.Getenv(EnvPort); p != "" {
		port = p
	}
	pharmABaseURL := os.Getenv(EnvPharmacyABaseURL)
	if pharmABaseURL == "" {
		return config{}, fmt.Errorf("pharmacy a base url is missing")
	}
	pharmASecret := os.Getenv(EnvPharmacyASecret)
	if pharmASecret == "" {
		return config{}, fmt.Errorf("pharmacy a secret is missing")
	}
	pharmBBaseURL := os.Getenv(EnvPharmacyBBaseURL)
	if pharmBBaseURL == "" {
		return config{}, fmt.Errorf("pharmacy b base url is missing")
	}
	pharmBSecret := os.Getenv(EnvPharmacyBSecret)
	if pharmBSecret == "" {
		return config{}, fmt.Errorf("pharmacy b secret is missing")
	}
	return config{
		DatabaseURL:      dbURL,
		JWTSecret:        []byte(jwtSecret),
		JWTTTL:           jwtTTL,
		Port:             port,
		PharmacyABaseURL: pharmABaseURL,
		PharmacyASecret:  pharmASecret,
		PharmacyBBaseURL: pharmBBaseURL,
		PharmacyBSecret:  pharmBSecret,
	}, nil
}

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
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("could not initialize database connection: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))
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

	if err := seedPharmacies(context.Background(), pharmacyRepo, pharmacySeeds); err != nil {
		return fmt.Errorf("error seeding pharmacy data: %w", err)
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

	// Inbound Adapters
	authHandler, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{
		Logger: logger,
		Svc:    userService,
	})
	if err != nil {
		return fmt.Errorf("failed to setup auth handler: %w", err)
	}

	api := httpapi.NewAPI(authHandler, tokenIssuer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	openapi.HandlerFromMux(api, mux)

	// Explicit timeouts guard against slow-client attacks (e.g. Slowloris);
	// the bare http.ListenAndServe uses a zero-value server with none, which
	// is what gosec flags (G114).
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
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

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}
