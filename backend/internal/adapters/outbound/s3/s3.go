// Package s3 is the adapter for using file storage in an s3 system
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/minio/minio-go/v7"
)

// S3 is the type for our s3 based file storage adapter
type S3 struct {
	client        *minio.Client
	presignClient *minio.Client
	bucket        string
	presignTTL    time.Duration
	logger        *slog.Logger
}

var _ outbound.FileStorage = (*S3)(nil)

// NewS3Params holds the params necessary to construct an S3 instance
type NewS3Params struct {
	Client        *minio.Client
	PresignClient *minio.Client
	Bucket        string
	PresignTTL    time.Duration
	Logger        *slog.Logger
}

// NewS3 is the constructor to build an S3 instance
func NewS3(p NewS3Params) (*S3, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.PresignClient == nil {
		return nil, errors.New("presign client is missing")
	}
	if p.Bucket == "" {
		return nil, errors.New("bucket is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}

	return &S3{
		client:        p.Client,
		presignClient: p.PresignClient,
		bucket:        p.Bucket,
		presignTTL:    p.PresignTTL,
		logger:        p.Logger,
	}, nil
}

// GetPresignedURL takes a file key and returns a secure, presigned url for
// retrieving a file
func (s *S3) GetPresignedURL(ctx context.Context, key string) (string, error) {
	url, err := s.presignClient.PresignedGetObject(
		ctx, s.bucket, key, s.presignTTL, nil,
	)
	if err != nil {
		return "", fmt.Errorf("error fetching presigned url: %w", err)
	}
	s.logger.DebugContext(ctx, "presigned URL issued", "key", key)
	return url.String(), nil
}

// Put takes a file key, content type, and an ioreader so it can store a file in
// an S3 system
func (s *S3) Put(
	ctx context.Context,
	key, contentType string,
	reader io.Reader,
) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	_, err := s.client.PutObject(
		ctx, s.bucket, key, reader, -1, opts,
	)
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}
	s.logger.DebugContext(ctx, "file successfully uploaded", "key", key)
	return nil
}
