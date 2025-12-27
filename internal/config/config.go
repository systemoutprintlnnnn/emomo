package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Qdrant    QdrantConfig    `mapstructure:"qdrant"`
	MinIO     MinIOConfig     `mapstructure:"minio"`
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
	Path string `mapstructure:"path"`
}

type QdrantConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
}

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Bucket    string `mapstructure:"bucket"`
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
	v.SetDefault("database.path", "./data/memes.db")
	v.SetDefault("qdrant.host", "localhost")
	v.SetDefault("qdrant.port", 6334)
	v.SetDefault("qdrant.collection", "memes")
	v.SetDefault("minio.endpoint", "localhost:9000")
	v.SetDefault("minio.use_ssl", false)
	v.SetDefault("minio.bucket", "memes")
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
	v.BindEnv("qdrant.host", "QDRANT_HOST")
	v.BindEnv("qdrant.port", "QDRANT_PORT")
	v.BindEnv("minio.endpoint", "MINIO_ENDPOINT")
	v.BindEnv("minio.access_key", "MINIO_ACCESS_KEY")
	v.BindEnv("minio.secret_key", "MINIO_SECRET_KEY")
	v.BindEnv("minio.use_ssl", "MINIO_USE_SSL")
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

	return &cfg, nil
}
