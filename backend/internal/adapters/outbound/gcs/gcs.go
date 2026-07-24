// Package gcs is the adapter specific to google cloud storage.
// It implements the file_storage port
package gcs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iamcredentials/v1"
)

// GCS is the type for the GCS storage adapter
type GCS struct {
	client         *storage.Client
	bucket         string
	logger         *slog.Logger
	presignTTL     time.Duration
	signingClient  *iamcredentials.Service
	googleAccessID string
}

var _ outbound.FileStorage = (*GCS)(nil)

// NewGCSParams holds the params necessary to construct the adapter
type NewGCSParams struct {
	Client         *storage.Client
	Bucket         string
	Logger         *slog.Logger
	PresignTTL     time.Duration
	SigningClient  *iamcredentials.Service
	GoogleAccessID string
}

// NewGCS constructs the adapter
func NewGCS(p NewGCSParams) (*GCS, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Bucket == "" {
		return nil, errors.New("bucket is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.SigningClient == nil {
		return nil, errors.New("signing client is missing")
	}
	if p.GoogleAccessID == "" {
		return nil, errors.New("google access id is missing")
	}
	return &GCS{
		client:         p.Client,
		bucket:         p.Bucket,
		logger:         p.Logger,
		presignTTL:     p.PresignTTL,
		signingClient:  p.SigningClient,
		googleAccessID: p.GoogleAccessID,
	}, nil
}

// GetPresignedURL takes a file key and returns a secure, presigned url for
// retrieving a file
func (s *GCS) GetPresignedURL(ctx context.Context, key string) (string, error) {
	signBytes := func(b []byte) ([]byte, error) {
		name := "projects/-/serviceAccounts/" + s.googleAccessID
		req := &iamcredentials.SignBlobRequest{
			Payload: base64.StdEncoding.EncodeToString(b),
		}
		resp, err := s.signingClient.Projects.ServiceAccounts.SignBlob(name, req).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("error signing blob: %w", err)
		}
		signature, err := base64.StdEncoding.DecodeString(resp.SignedBlob)
		if err != nil {
			return nil, fmt.Errorf("error decoding blob: %w", err)
		}
		return signature, nil
	}
	opts := storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         "GET",
		Expires:        time.Now().Add(s.presignTTL),
		GoogleAccessID: s.googleAccessID,
		SignBytes:      signBytes,
	}
	url, err := s.client.Bucket(s.bucket).SignedURL(key, &opts)
	if err != nil {
		return "", fmt.Errorf("error generating signed url: %w", err)
	}
	s.logger.DebugContext(ctx, "presigned URL issued", "key", key)
	return url, nil
}

// Put takes a file key, content type, and an ioreader so it can store a file in
// GCS
func (s *GCS) Put(
	ctx context.Context,
	key, contentType string,
	reader io.Reader,
) error {
	handle := s.client.Bucket(s.bucket).Object(key)
	writer := handle.NewWriter(ctx)
	writer.ContentType = contentType

	_, err := io.Copy(writer, reader)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("error copying file: %w", err)
	}
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("error closing stream: %w", err)
	}
	s.logger.DebugContext(ctx, "file successfully uploaded", "key", key)
	return nil
}
