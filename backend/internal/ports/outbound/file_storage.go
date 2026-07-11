package outbound

import (
	"context"
	"io"
)

// FileStorage is the port for storing uploaded prescription files
type FileStorage interface {
	GetPresignedURL(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, contentType string, reader io.Reader) error
}
