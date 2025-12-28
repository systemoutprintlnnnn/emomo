package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Qdrant    QdrantConfig    `mapstructure:"qdrant"`
	Storage   StorageConfig   `mapstructure:"storage"`
	VLM       VLMConfig       `mapstructure:"vlm"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Ingest    IngestConfig    `mapstructure:"ingest"`
	Sources   SourcesConfig   `mapstructure:"sources"`
	Search    SearchConfig    `mapstructure:"search"`
}

type ServerConfig struct {
	Port int        `mapstructure:"port"`
	Mode string     `mapstructure:"mode"`
	CORS CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	AllowAllOrigins bool     `mapstructure:"allow_all_origins"`
}

type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`          // Database driver: sqlite, postgres
	URL             string        `mapstructure:"url"`             // PostgreSQL connection URL (takes priority)
	Path            string        `mapstructure:"path"`            // SQLite path
	Host            string        `mapstructure:"host"`            // PostgreSQL host
	Port            int           `mapstructure:"port"`            // PostgreSQL port
	User            string        `mapstructure:"user"`            // PostgreSQL user
	Password        string        `mapstructure:"password"`        // PostgreSQL password
	DBName          string        `mapstructure:"dbname"`          // PostgreSQL db name
	SSLMode         string        `mapstructure:"sslmode"`         // PostgreSQL sslmode
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`  // Connection pool: max idle
	MaxOpenConns    int           `mapstructure:"max_open_conns"`  // Connection pool: max open
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // Connection pool: max lifetime
}

// DSN returns the Data Source Name for the database connection
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

type QdrantConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
	APIKey     string `mapstructure:"api_key"` // Qdrant Cloud API Key
	UseTLS     bool   `mapstructure:"use_tls"` // Enable TLS (auto-enabled when APIKey is set)
}

// StorageConfig holds configuration for S3-compatible storage (R2, S3, etc.)
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

type VLMConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
}

type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`
	Model      string `mapstructure:"model"`
	APIKey     string `mapstructure:"api_key"`
	Dimensions int    `mapstructure:"dimensions"`
}

type IngestConfig struct {
	Workers    int `mapstructure:"workers"`
	BatchSize  int `mapstructure:"batch_size"`
	RetryCount int `mapstructure:"retry_count"`
}

type SearchConfig struct {
	ScoreThreshold float32              `mapstructure:"score_threshold"`
	QueryExpansion QueryExpansionConfig `mapstructure:"query_expansion"`
}

type QueryExpansionConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Model   string `mapstructure:"model"`
}

type SourcesConfig struct {
	ChineseBQB ChineseBQBConfig `mapstructure:"chinesebqb"`
}

type ChineseBQBConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	RepoPath string `mapstructure:"repo_path"`
}

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
	v.SetDefault("vlm.provider", "openai")
	v.SetDefault("vlm.model", "gpt-4o-mini")
	v.SetDefault("vlm.base_url", "https://api.openai.com/v1")
	v.SetDefault("embedding.provider", "jina")
	v.SetDefault("embedding.model", "jina-embeddings-v3")
	v.SetDefault("embedding.dimensions", 1024)
	v.SetDefault("ingest.workers", 5)
	v.SetDefault("ingest.batch_size", 10)
	v.SetDefault("ingest.retry_count", 3)
	v.SetDefault("sources.chinesebqb.enabled", true)
	v.SetDefault("sources.chinesebqb.repo_path", "./data/ChineseBQB")
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
	v.BindEnv("embedding.api_key", "JINA_API_KEY")
	v.BindEnv("search.score_threshold", "SEARCH_SCORE_THRESHOLD")
	v.BindEnv("search.query_expansion.model", "QUERY_EXPANSION_MODEL")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Log database driver for debugging
	fmt.Printf("[Config] Database driver: %q (env DATABASE_DRIVER=%q)\n", cfg.Database.Driver, os.Getenv("DATABASE_DRIVER"))

	return &cfg, nil
}

// GetStorageConfig returns the storage configuration.
func (c *Config) GetStorageConfig() *StorageConfig {
	return &c.Storage
}
