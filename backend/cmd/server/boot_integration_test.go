//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/LoneWolfPR/MedMarket/backend/ent"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	temporalclient "go.temporal.io/sdk/client"
)

// TestBuildHandlerBoot exercises the real composition root: it drives the graph
// buildHandler assembles (the same one run() serves in production) end-to-end
// over HTTP, against a throwaway Postgres. It is the one test that proves main's
// wiring actually boots and serves — every layer below is covered elsewhere with
// fakes, so this only asserts the seams line up.
//
// Only the auth slice and health are exercised: those routes touch Postgres but
// not Temporal/MinIO/the mock pharmacies, so the Temporal client can stay lazy
// (never dials) and MinIO is never contacted. The heavier flows (upload, search,
// order, shipping) belong to the compose smoke test, which has the real infra.
func TestBuildHandlerBoot(t *testing.T) {
	client := newBootTestClient(t)
	tc, err := temporalclient.NewLazyClient(temporalclient.Options{
		HostPort:  "localhost:7233",
		Namespace: "default",
	})
	require.NoError(t, err)
	t.Cleanup(func() { tc.Close() })

	handler, err := buildHandler(buildHandlerParams{
		Logger:         slog.New(slog.DiscardHandler),
		Config:         bootTestConfig(),
		EntClient:      client,
		TemporalClient: tc,
	})
	require.NoError(t, err)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	t.Run("health", func(t *testing.T) {
		status, _ := get(t, srv.URL+"/api/health", "")
		assert.Equal(t, http.StatusOK, status)
	})

	t.Run("auth flow", func(t *testing.T) {
		email := "boot-" + uuid.NewString() + "@example.com"
		const password = "Test1234!"

		// register
		status, _ := postJSON(t, srv.URL+"/api/auth/register", "", map[string]any{
			"firstName": "Test",
			"lastName":  "User",
			"email":     email,
			"password":  password,
		})
		require.Equal(t, http.StatusCreated, status)

		// login -> token
		status, body := postJSON(t, srv.URL+"/api/auth/login", "", map[string]any{
			"email":    email,
			"password": password,
		})
		require.Equal(t, http.StatusOK, status)
		var tok struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.Unmarshal(body, &tok))
		require.NotEmpty(t, tok.Token)

		// profile with the token succeeds
		status, _ = get(t, srv.URL+"/api/auth/profile", tok.Token)
		assert.Equal(t, http.StatusOK, status)

		// profile without a token is rejected by the auth middleware
		status, _ = get(t, srv.URL+"/api/auth/profile", "")
		assert.Equal(t, http.StatusUnauthorized, status)
	})
}

// bootTestConfig returns a config valid enough for buildHandler's constructors:
// non-empty JWT secret / TTL and webhook URL (each is validated), and MinIO
// coordinates that let minio.New construct without dialing. The Temporal and
// pharmacy fields are unused by buildHandler (their adapters live in the worker,
// and the client is injected), so they are left zero.
func bootTestConfig() config {
	return config{
		JWTSecret:           []byte("boot-test-secret"),
		JWTTTL:              time.Hour,
		Port:                "8080",
		MinioEndpoint:       "localhost:9000",
		MinioPublicEndpoint: "localhost:9000",
		MinioRegion:         "us-east-1",
		MinioAccessKey:      "test",
		MinioSecretKey:      "testtest",
		MinioBucket:         "test",
		MinioUseSSL:         false,
		OfferTTL:            time.Hour,
		WebhookBaseURL:      "http://localhost:8080",
	}
}

// newBootTestClient spins up a throwaway Postgres, builds the Ent schema into it,
// and returns a client wired to it, all torn down via t.Cleanup. It mirrors the
// postgres package's integration helper; the two live in different packages so
// the helper cannot be shared.
func newBootTestClient(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("medmarket_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx))
	return client
}

// get issues a GET, attaching a bearer token when non-empty, and returns the
// status and fully-read body.
func get(t *testing.T, url, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	return do(t, req, token)
}

// postJSON issues a POST with a JSON body, attaching a bearer token when
// non-empty, and returns the status and fully-read body.
func postJSON(t *testing.T, url, token string, body any) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return do(t, req, token)
}

// do sends req, reads and closes the body, and returns the status and body bytes.
// Reading the whole body before closing lets the transport reuse the connection.
func do(t *testing.T, req *http.Request, token string) (int, []byte) {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}
