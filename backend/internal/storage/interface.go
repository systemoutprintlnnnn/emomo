package storage

import (
	"context"
	"io"
)

// ObjectStorage defines the interface for object storage operations.
type ObjectStorage interface {
	// EnsureBucket ensures the configured bucket exists.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	// Returns:
	//   - error: non-nil if the bucket check/create fails.
	EnsureBucket(ctx context.Context) error

	// Upload stores an object at the given key.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	//   - key: storage key (path) for the object.
	//   - reader: stream providing the object content.
	//   - size: content length in bytes.
	//   - contentType: MIME type for the object.
	// Returns:
	//   - error: non-nil if the upload fails.
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download retrieves an object by key.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	//   - key: storage key (path) for the object.
	// Returns:
	//   - io.ReadCloser: reader for the object contents.
	//   - error: non-nil if the download fails.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// GetURL returns a public or signed URL for the object.
	// Parameters:
	//   - key: storage key (path) for the object.
	// Returns:
	//   - string: URL that can be used to access the object.
	GetURL(key string) string

	// Delete removes an object by key.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	//   - key: storage key (path) for the object.
	// Returns:
	//   - error: non-nil if the delete fails.
	Delete(ctx context.Context, key string) error

	// Exists checks if an object exists by key.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	//   - key: storage key (path) for the object.
	// Returns:
	//   - bool: true if the object exists.
	//   - error: non-nil if the check fails.
	Exists(ctx context.Context, key string) (bool, error)
}
