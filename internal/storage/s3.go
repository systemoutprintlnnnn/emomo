package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// StorageType defines the type of S3-compatible storage
type StorageType string

const (
	StorageTypeMinio StorageType = "minio"
	StorageTypeR2    StorageType = "r2"
	StorageTypeS3    StorageType = "s3"
)

// S3Config holds configuration for S3-compatible storage
type S3Config struct {
	Type      StorageType
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
	Region    string
	PublicURL string // Public URL prefix for R2.dev or custom CDN
}

// S3Storage implements ObjectStorage for S3-compatible services
type S3Storage struct {
	client    *minio.Client
	bucket    string
	endpoint  string
	useSSL    bool
	storeType StorageType
	publicURL string
	region    string
}

// NewS3Storage creates a new S3-compatible storage client
func NewS3Storage(cfg *S3Config) (*S3Storage, error) {
	// Normalize endpoint: remove protocol prefix and trailing slashes/paths
	endpoint := normalizeEndpoint(cfg.Endpoint)

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	}

	// Set region based on storage type
	switch cfg.Type {
	case StorageTypeR2:
		opts.Region = "auto" // R2 uses "auto" region
	case StorageTypeS3:
		if cfg.Region != "" {
			opts.Region = cfg.Region
		}
	}

	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Normalize public URL (remove trailing slash)
	publicURL := strings.TrimSuffix(cfg.PublicURL, "/")

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		endpoint:  endpoint,
		useSSL:    cfg.UseSSL,
		storeType: cfg.Type,
		publicURL: publicURL,
		region:    cfg.Region,
	}, nil
}

// normalizeEndpoint removes protocol prefix and path from endpoint
func normalizeEndpoint(endpoint string) string {
	// Remove protocol prefix
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	// Remove any path (everything after the first /)
	if idx := strings.Index(endpoint, "/"); idx != -1 {
		endpoint = endpoint[:idx]
	}

	// Remove trailing slashes
	endpoint = strings.TrimSuffix(endpoint, "/")

	return endpoint
}

// EnsureBucket creates the bucket if it doesn't exist
func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists {
		return nil
	}

	// R2 doesn't support creating buckets via API - must use dashboard
	if s.storeType == StorageTypeR2 {
		return fmt.Errorf("bucket %s does not exist, please create it in R2 dashboard", s.bucket)
	}

	// Create bucket for MinIO and S3
	opts := minio.MakeBucketOptions{}
	if s.region != "" {
		opts.Region = s.region
	}

	if err := s.client.MakeBucket(ctx, s.bucket, opts); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	// Set bucket policy to allow public read access (MinIO only)
	if s.storeType == StorageTypeMinio {
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/*"]
				}
			]
		}`, s.bucket)

		if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
			// Non-fatal: bucket is created, just can't set public policy
			fmt.Printf("Warning: failed to set bucket policy: %v\n", err)
		}
	}

	return nil
}

// Upload uploads an object to storage
func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, opts)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads an object from storage
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download object: %w", err)
	}

	return obj, nil
}

// GetURL returns the public URL for accessing an object
func (s *S3Storage) GetURL(key string) string {
	// If public URL is configured (R2.dev or custom CDN), use it
	if s.publicURL != "" {
		return fmt.Sprintf("%s/%s", s.publicURL, key)
	}

	// Generate URL based on storage type
	switch s.storeType {
	case StorageTypeS3:
		// AWS S3 virtual-hosted style URL
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	case StorageTypeR2:
		// R2 without public URL configured - use S3 API endpoint (requires signing)
		// Note: It's recommended to always configure public_url for R2
		scheme := "http"
		if s.useSSL {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, key)
	default:
		// MinIO and other compatible services
		scheme := "http"
		if s.useSSL {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, key)
	}
}

// Delete deletes an object from storage
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// Exists checks if an object exists in storage
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}
