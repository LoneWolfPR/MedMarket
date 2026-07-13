package s3_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/s3"
)

// newMinioClient builds a minio client. minio.New does not dial, so this is
// cheap and needs no running server — enough to satisfy the constructor's
// non-nil guards.
func newMinioClient(t *testing.T) *minio.Client {
	t.Helper()

	c, err := minio.New("localhost:9000", &minio.Options{
		Creds: credentials.NewStaticV4("key", "secret", ""),
	})
	require.NoError(t, err)
	return c
}

func TestNewS3_Validation(t *testing.T) {
	client := newMinioClient(t)
	presign := newMinioClient(t)
	logger := slog.New(slog.DiscardHandler)

	tests := map[string]s3.NewS3Params{
		"missing client":         {PresignClient: presign, Bucket: "b", Logger: logger},
		"missing presign client": {Client: client, Bucket: "b", Logger: logger},
		"missing bucket":         {Client: client, PresignClient: presign, Logger: logger},
		"missing logger":         {Client: client, PresignClient: presign, Bucket: "b"},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := s3.NewS3(params)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestNewS3_Valid(t *testing.T) {
	got, err := s3.NewS3(s3.NewS3Params{
		Client:        newMinioClient(t),
		PresignClient: newMinioClient(t),
		Bucket:        "prescriptions",
		PresignTTL:    15 * time.Minute,
		Logger:        slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	assert.NotNil(t, got)
}
