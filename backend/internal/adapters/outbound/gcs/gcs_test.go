package gcs_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/gcs"
)

const (
	testBucket   = "prescriptions"
	testAccessID = "medmarket-storage@example.iam.gserviceaccount.com"
	testKey      = "user-1/doc.pdf"
)

// newStorageClient builds a storage client without credentials. SignedURL is an
// offline computation (its only network call is delegated to SignBytes), so no
// running GCS endpoint is needed — this just satisfies the constructor.
func newStorageClient(t *testing.T) *storage.Client {
	t.Helper()
	c, err := storage.NewClient(context.Background(), option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newSigningClient points an iamcredentials service at the given stub server so
// the SignBytes closure's signBlob call is exercised without hitting real IAM.
func newSigningClient(t *testing.T, srvURL string, httpClient *http.Client) *iamcredentials.Service {
	t.Helper()
	svc, err := iamcredentials.NewService(
		context.Background(),
		option.WithEndpoint(srvURL),
		option.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)
	return svc
}

// newSigningClientOffline builds a signing client with no endpoint override for
// tests that never issue a signBlob call (e.g. constructor validation).
func newSigningClientOffline(t *testing.T) *iamcredentials.Service {
	t.Helper()
	svc, err := iamcredentials.NewService(context.Background(), option.WithoutAuthentication())
	require.NoError(t, err)
	return svc
}

func TestNewGCS_Validation(t *testing.T) {
	valid := gcs.NewGCSParams{
		Client:         newStorageClient(t),
		Bucket:         testBucket,
		Logger:         slog.New(slog.DiscardHandler),
		SigningClient:  newSigningClientOffline(t),
		GoogleAccessID: testAccessID,
	}

	// each case starts from valid params and clears exactly one required field.
	tests := map[string]func(p *gcs.NewGCSParams){
		"missing client":         func(p *gcs.NewGCSParams) { p.Client = nil },
		"missing bucket":         func(p *gcs.NewGCSParams) { p.Bucket = "" },
		"missing logger":         func(p *gcs.NewGCSParams) { p.Logger = nil },
		"missing signing client": func(p *gcs.NewGCSParams) { p.SigningClient = nil },
		"missing access id":      func(p *gcs.NewGCSParams) { p.GoogleAccessID = "" },
	}
	for name, breakOne := range tests {
		t.Run(name, func(t *testing.T) {
			params := valid
			breakOne(&params)
			got, err := gcs.NewGCS(params)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestNewGCS_Valid(t *testing.T) {
	got, err := gcs.NewGCS(gcs.NewGCSParams{
		Client:         newStorageClient(t),
		Bucket:         testBucket,
		Logger:         slog.New(slog.DiscardHandler),
		PresignTTL:     15 * time.Minute,
		SigningClient:  newSigningClientOffline(t),
		GoogleAccessID: testAccessID,
	})
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestGetPresignedURL(t *testing.T) {
	var (
		called       bool
		gotPath      string
		payloadIsB64 bool
	)
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotPath = r.URL.Path

		var body iamcredentials.SignBlobRequest
		// assert (not require) — this runs in the server goroutine, where
		// FailNow is unsafe.
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, decErr := base64.StdEncoding.DecodeString(body.Payload)
		payloadIsB64 = decErr == nil

		sig := base64.StdEncoding.EncodeToString([]byte("fake-signature"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"signedBlob":"` + sig + `"}`))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	adapter, err := gcs.NewGCS(gcs.NewGCSParams{
		Client:         newStorageClient(t),
		Bucket:         testBucket,
		Logger:         slog.New(slog.DiscardHandler),
		PresignTTL:     15 * time.Minute,
		SigningClient:  newSigningClient(t, srv.URL, srv.Client()),
		GoogleAccessID: testAccessID,
	})
	require.NoError(t, err)

	url, err := adapter.GetPresignedURL(context.Background(), testKey)
	require.NoError(t, err)

	// Our logic: signBlob was called on the right SA with a base64 payload.
	assert.True(t, called, "signBlob endpoint should be called")
	assert.True(t, payloadIsB64, "payload should be base64-encoded")
	assert.Contains(t, gotPath, "projects/-/serviceAccounts/"+testAccessID)

	// The resulting URL is a V4 signed GET against the bucket/object.
	assert.Contains(t, url, "storage.googleapis.com")
	assert.Contains(t, url, testBucket)
	assert.Contains(t, url, "X-Goog-Algorithm=GOOG4-RSA-SHA256")
}

func TestGetPresignedURL_SignBlobError(t *testing.T) {
	// 400 (not 500) so the api client doesn't retry with backoff.
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"boom"}}`))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	adapter, err := gcs.NewGCS(gcs.NewGCSParams{
		Client:         newStorageClient(t),
		Bucket:         testBucket,
		Logger:         slog.New(slog.DiscardHandler),
		PresignTTL:     15 * time.Minute,
		SigningClient:  newSigningClient(t, srv.URL, srv.Client()),
		GoogleAccessID: testAccessID,
	})
	require.NoError(t, err)

	url, err := adapter.GetPresignedURL(context.Background(), testKey)
	require.Error(t, err)
	assert.Empty(t, url)
}
