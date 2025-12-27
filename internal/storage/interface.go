package storage

import (
	"context"
	"io"
)

// ObjectStorage defines the interface for object storage operations
type ObjectStorage interface {
	// EnsureBucket creates the bucket if it doesn't exist
	EnsureBucket(ctx context.Context) error

	// Upload uploads an object to storage
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download downloads an object from storage
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// GetURL returns the URL for accessing an object
	GetURL(key string) string

	// Delete deletes an object from storage
	Delete(ctx context.Context, key string) error

	// Exists checks if an object exists
	Exists(ctx context.Context, key string) (bool, error)
}
