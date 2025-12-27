package storage

import "strings"

// NewStorage creates an ObjectStorage instance based on the configuration
func NewStorage(cfg *S3Config) (ObjectStorage, error) {
	// Auto-detect storage type if not specified
	if cfg.Type == "" {
		cfg.Type = detectStorageType(cfg.Endpoint)
	}

	return NewS3Storage(cfg)
}

// detectStorageType attempts to detect the storage type from the endpoint
func detectStorageType(endpoint string) StorageType {
	endpoint = strings.ToLower(endpoint)

	switch {
	case strings.Contains(endpoint, "r2.cloudflarestorage.com"):
		return StorageTypeR2
	case strings.Contains(endpoint, "amazonaws.com"):
		return StorageTypeS3
	default:
		return StorageTypeMinio
	}
}
