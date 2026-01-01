package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config aggregates application configuration loaded from files and environment.
type Config struct {
	Server     ServerConfig               `mapstructure:"server"`
	Database   DatabaseConfig             `mapstructure:"database"`
	Qdrant     QdrantConfig               `mapstructure:"qdrant"`
	Storage    StorageConfig              `mapstructure:"storage"`
	VLM        VLMConfig                  `mapstructure:"vlm"`
	Embedding  EmbeddingConfig            `mapstructure:"embedding"`  // Default embedding config
	Embeddings map[string]EmbeddingConfig `mapstructure:"embeddings"` // Named embedding configs
	Ingest     IngestConfig               `mapstructure:"ingest"`
	Sources    SourcesConfig              `mapstructure:"sources"`
	Search     SearchConfig               `mapstructure:"search"`
}

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Port int        `mapstructure:"port"`
	Mode string     `mapstructure:"mode"`
	CORS CORSConfig `mapstructure:"cors"`
}

// CORSConfig defines Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	AllowAllOrigins bool     `mapstructure:"allow_all_origins"`
}

// DatabaseConfig defines database connection and pool settings.
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`            // Database driver: sqlite, postgres
	URL             string        `mapstructure:"url"`               // PostgreSQL connection URL (takes priority)
	Path            string        `mapstructure:"path"`              // SQLite path
	Host            string        `mapstructure:"host"`              // PostgreSQL host
	Port            int           `mapstructure:"port"`              // PostgreSQL port
	User            string        `mapstructure:"user"`              // PostgreSQL user
	Password        string        `mapstructure:"password"`          // PostgreSQL password
	DBName          string        `mapstructure:"dbname"`            // PostgreSQL db name
	SSLMode         string        `mapstructure:"sslmode"`           // PostgreSQL sslmode
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`    // Connection pool: max idle
	MaxOpenConns    int           `mapstructure:"max_open_conns"`    // Connection pool: max open
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // Connection pool: max lifetime
}

// DSN builds the Data Source Name for the configured database.
// Parameters: none.
// Returns:
//   - string: DSN string suitable for GORM drivers.
func (c *DatabaseConfig) DSN() string {
	if c.Driver == "sqlite" {
		return c.Path
	}

	// If URL is explicitly provided, use it
	if c.URL != "" {
		return c.URL
	}

	// Build PostgreSQL DSN
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
	return dsn
}

// QdrantConfig defines Qdrant connection settings.
type QdrantConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
	APIKey     string `mapstructure:"api_key"` // Qdrant Cloud API Key
	UseTLS     bool   `mapstructure:"use_tls"` // Enable TLS (auto-enabled when APIKey is set)
}

// StorageConfig holds configuration for S3-compatible storage (R2, S3, etc.).
type StorageConfig struct {
	Type      string `mapstructure:"type"`       // "r2", "s3", "s3compatible"
	Endpoint  string `mapstructure:"endpoint"`   // S3 API endpoint
	AccessKey string `mapstructure:"access_key"` // Access key ID
	SecretKey string `mapstructure:"secret_key"` // Secret access key
	UseSSL    bool   `mapstructure:"use_ssl"`    // Use HTTPS
	Bucket    string `mapstructure:"bucket"`     // Bucket name
	Region    string `mapstructure:"region"`     // Region (for AWS S3)
	PublicURL string `mapstructure:"public_url"` // Public URL prefix (e.g., R2.dev domain)
}

// VLMConfig defines configuration for the Vision Language Model provider.
type VLMConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
}

// EmbeddingConfig defines configuration for a text embedding provider.
type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`   // "jina", "modelscope", "openai-compatible"
	Model      string `mapstructure:"model"`      // Model name/ID
	APIKey     string `mapstructure:"api_key"`    // API key for authentication
	BaseURL    string `mapstructure:"base_url"`   // Base URL for OpenAI-compatible APIs
	Dimensions int    `mapstructure:"dimensions"` // Embedding vector dimensions
	Collection string `mapstructure:"collection"` // Qdrant collection name for this embedding
}

// IngestConfig defines ingestion concurrency and batching settings.
type IngestConfig struct {
	Workers    int `mapstructure:"workers"`
	BatchSize  int `mapstructure:"batch_size"`
	RetryCount int `mapstructure:"retry_count"`
}

// SearchConfig defines search runtime settings.
type SearchConfig struct {
	ScoreThreshold float32              `mapstructure:"score_threshold"`
	QueryExpansion QueryExpansionConfig `mapstructure:"query_expansion"`
}

// QueryExpansionConfig configures optional LLM-based query expansion.
type QueryExpansionConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Model   string `mapstructure:"model"`
}

// SourcesConfig defines configuration for available data sources.
type SourcesConfig struct {
	ChineseBQB ChineseBQBConfig `mapstructure:"chinesebqb"`
	Staging    StagingConfig    `mapstructure:"staging"`
}

// ChineseBQBConfig defines configuration for the ChineseBQB source.
type ChineseBQBConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	RepoPath string `mapstructure:"repo_path"`
}

// StagingConfig defines configuration for the staging source.
type StagingConfig struct {
	Path string `mapstructure:"path"` // Base path for staging directory
}

// Load reads configuration from file/environment and returns a Config.
// Parameters:
//   - configPath: optional explicit path to a config file.
// Returns:
//   - *Config: loaded configuration with defaults applied.
//   - error: non-nil if loading or unmarshalling fails.
func Load(configPath string) (*Config, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	v := viper.New()

	// Set config file path
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Enable environment variable override
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.cors.allow_all_origins", true)
	v.SetDefault("server.cors.allowed_origins", []string{})
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "./data/memes.db")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "emomo")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("qdrant.host", "localhost")
	v.SetDefault("qdrant.port", 6334)
	v.SetDefault("qdrant.collection", "emomo")
	v.SetDefault("qdrant.api_key", "")
	v.SetDefault("qdrant.use_tls", false)
	v.SetDefault("storage.endpoint", "localhost:9000")
	v.SetDefault("storage.use_ssl", false)
	v.SetDefault("storage.bucket", "memes")
	// VLM Configuration (OpenAI Compatible)
	v.SetDefault("vlm.provider", "openai")
	v.SetDefault("vlm.model", "gpt-4o-mini") // Can be any compatible model
	v.SetDefault("vlm.base_url", "https://api.openai.com/v1")
	v.SetDefault("embedding.provider", "jina")
	v.SetDefault("embedding.model", "jina-embeddings-v3")
	v.SetDefault("embedding.dimensions", 1024)
	v.SetDefault("ingest.workers", 5)
	v.SetDefault("ingest.batch_size", 10)
	v.SetDefault("ingest.retry_count", 3)
	v.SetDefault("sources.chinesebqb.enabled", true)
	v.SetDefault("sources.chinesebqb.repo_path", "./data/ChineseBQB")
	v.SetDefault("sources.staging.path", "./data/staging")
	v.SetDefault("search.score_threshold", 0.0)
	v.SetDefault("search.query_expansion.enabled", true)
	v.SetDefault("search.query_expansion.model", "gpt-4o-mini")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Bind environment variables explicitly for sensitive data
	v.BindEnv("server.port", "PORT") // Hugging Face Spaces uses PORT env var (default: 7860)

	// Database environment variables
	v.BindEnv("database.driver", "DATABASE_DRIVER")
	v.BindEnv("database.url", "DATABASE_URL") // PostgreSQL connection URL (takes priority)
	v.BindEnv("database.path", "DATABASE_PATH")
	v.BindEnv("database.host", "DATABASE_HOST")
	v.BindEnv("database.port", "DATABASE_PORT")
	v.BindEnv("database.user", "DATABASE_USER")
	v.BindEnv("database.password", "DATABASE_PASSWORD")
	v.BindEnv("database.dbname", "DATABASE_DBNAME")
	v.BindEnv("database.sslmode", "DATABASE_SSLMODE")
	v.BindEnv("database.sslrootcert", "DATABASE_SSLROOTCERT")

	v.BindEnv("qdrant.host", "QDRANT_HOST")
	v.BindEnv("qdrant.port", "QDRANT_PORT")
	v.BindEnv("qdrant.collection", "QDRANT_COLLECTION")
	v.BindEnv("qdrant.api_key", "QDRANT_API_KEY")
	v.BindEnv("qdrant.use_tls", "QDRANT_USE_TLS")

	// Storage environment variables
	v.BindEnv("storage.type", "STORAGE_TYPE")
	v.BindEnv("storage.endpoint", "STORAGE_ENDPOINT")
	v.BindEnv("storage.access_key", "STORAGE_ACCESS_KEY")
	v.BindEnv("storage.secret_key", "STORAGE_SECRET_KEY")
	v.BindEnv("storage.use_ssl", "STORAGE_USE_SSL")
	v.BindEnv("storage.bucket", "STORAGE_BUCKET")
	v.BindEnv("storage.region", "STORAGE_REGION")
	v.BindEnv("storage.public_url", "STORAGE_PUBLIC_URL")
	v.BindEnv("vlm.api_key", "OPENAI_API_KEY")
	v.BindEnv("vlm.base_url", "OPENAI_BASE_URL")
	v.BindEnv("vlm.model", "VLM_MODEL")

	// Default embedding environment variables
	v.BindEnv("embedding.provider", "EMBEDDING_PROVIDER")
	v.BindEnv("embedding.model", "EMBEDDING_MODEL")
	v.BindEnv("embedding.dimensions", "EMBEDDING_DIMENSIONS")
	v.BindEnv("embedding.api_key", "EMBEDDING_API_KEY")
	v.BindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	v.BindEnv("embedding.collection", "EMBEDDING_COLLECTION")

	// Named embeddings environment variables
	v.BindEnv("embeddings.jina.api_key", "JINA_API_KEY")
	v.BindEnv("embeddings.qwen3.api_key", "MODELSCOPE_API_KEY")
	v.BindEnv("embeddings.qwen3.base_url", "MODELSCOPE_BASE_URL")

	v.BindEnv("search.score_threshold", "SEARCH_SCORE_THRESHOLD")
	v.BindEnv("search.query_expansion.model", "QUERY_EXPANSION_MODEL")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Manually apply environment variable overrides for map-based embedding configs
	// (Viper's BindEnv doesn't work well with map keys)
	if cfg.Embeddings != nil {
		// Override qwen3 config from environment
		if qwen3Cfg, ok := cfg.Embeddings["qwen3"]; ok {
			if apiKey := os.Getenv("MODELSCOPE_API_KEY"); apiKey != "" {
				qwen3Cfg.APIKey = apiKey
			}
			if baseURL := os.Getenv("MODELSCOPE_BASE_URL"); baseURL != "" {
				qwen3Cfg.BaseURL = baseURL
			}
			cfg.Embeddings["qwen3"] = qwen3Cfg
		}
		// Override jina config from environment
		if jinaCfg, ok := cfg.Embeddings["jina"]; ok {
			if apiKey := os.Getenv("JINA_API_KEY"); apiKey != "" {
				jinaCfg.APIKey = apiKey
			}
			cfg.Embeddings["jina"] = jinaCfg
		}
	}

	// Log database driver for debugging
	fmt.Printf("[Config] Database driver: %q (env DATABASE_DRIVER=%q)\n", cfg.Database.Driver, os.Getenv("DATABASE_DRIVER"))

	return &cfg, nil
}

// GetStorageConfig returns the storage configuration.
// Parameters: none.
// Returns:
//   - *StorageConfig: pointer to the storage config section.
func (c *Config) GetStorageConfig() *StorageConfig {
	return &c.Storage
}

// GetEmbeddingConfig returns the embedding configuration by name.
// Parameters:
//   - name: embedding config name; empty uses the default embedding.
// Returns:
//   - *EmbeddingConfig: config for the named embedding, or nil if missing.
func (c *Config) GetEmbeddingConfig(name string) *EmbeddingConfig {
	if name == "" {
		return &c.Embedding
	}
	if cfg, ok := c.Embeddings[name]; ok {
		return &cfg
	}
	return nil
}

// GetCollectionForEmbedding returns the Qdrant collection name for an embedding.
// Parameters:
//   - embeddingName: embedding config name; empty uses the default embedding.
// Returns:
//   - string: collection name to use for the embedding.
func (c *Config) GetCollectionForEmbedding(embeddingName string) string {
	if embeddingName == "" {
		// Default embedding uses default collection
		if c.Embedding.Collection != "" {
			return c.Embedding.Collection
		}
		return c.Qdrant.Collection
	}

	if cfg, ok := c.Embeddings[embeddingName]; ok {
		if cfg.Collection != "" {
			return cfg.Collection
		}
	}

	return c.Qdrant.Collection
}
