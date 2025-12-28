package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StorageType defines the type of S3-compatible storage
type StorageType string

const (
	StorageTypeR2  StorageType = "r2"
	StorageTypeS3  StorageType = "s3"
	StorageTypeS3Compatible StorageType = "s3compatible"
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
	client    *s3.Client
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

	// Determine region
	region := cfg.Region
	if region == "" {
		if cfg.Type == StorageTypeR2 {
			region = "auto"
		} else {
			region = "us-east-1" // Default region for S3-compatible services
		}
	}

	// Build endpoint URL
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s", scheme, endpoint)

	// Create AWS config
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
		o.UsePathStyle = true // Use path-style for S3-compatible services
	})

	// Normalize public URL (remove trailing slash)
	publicURL := strings.TrimSuffix(cfg.PublicURL, "/")

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		endpoint:  endpoint,
		useSSL:    cfg.UseSSL,
		storeType: cfg.Type,
		publicURL: publicURL,
		region:    region,
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
	// Check if bucket exists
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	// R2 doesn't support creating buckets via API - must use dashboard
	if s.storeType == StorageTypeR2 {
		return fmt.Errorf("bucket %s does not exist, please create it in R2 dashboard", s.bucket)
	}

	// Create bucket for S3 and S3-compatible services
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// Upload uploads an object to storage
func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads an object from storage
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download object: %w", err)
	}

	return result.Body, nil
}

// GetURL returns the public URL for accessing an object
func (s *S3Storage) GetURL(key string) string {
	return fmt.Sprintf("%s/%s", s.publicURL, key)
}

// Delete deletes an object from storage
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// Exists checks if an object exists in storage
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}