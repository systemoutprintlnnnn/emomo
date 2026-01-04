package logger

import (
	"io"
	"os"
	"strconv"
)

// EnvConfig holds extended logger configuration loaded from environment variables.
type EnvConfig struct {
	// Basic configuration
	Level       string    // Log level: debug, info, warn, error
	Format      string    // Output format: json, text
	Output      io.Writer // Output destination (highest priority)
	ServiceName string    // Service name for log tagging

	// Environment configuration
	Environment string // Environment: local, dev, prod

	// File output configuration
	LogFile     string // Log file path
	LogFileOnly bool   // Output only to file (not stdout)

	// Log rotation configuration
	MaxSize    int  // Max file size in MB before rotation
	MaxBackups int  // Number of backup files to keep
	MaxAge     int  // Max days to keep backup files
	Compress   bool // Compress rotated files
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() *EnvConfig {
	return &EnvConfig{
		Level:       getEnv("LOG_LEVEL", "info"),
		Format:      getEnv("LOG_FORMAT", "json"),
		ServiceName: getEnv("SERVICE_NAME", "emomo"),
		Environment: getEnv("APP_ENV", "local"),

		LogFile:     getEnv("LOG_FILE", "/var/log/emomo/app.log"),
		LogFileOnly: getEnvBool("LOG_FILE_ONLY", false),

		MaxSize:    getEnvInt("LOG_MAX_SIZE", 100),
		MaxBackups: getEnvInt("LOG_MAX_BACKUPS", 7),
		MaxAge:     getEnvInt("LOG_MAX_AGE", 30),
		Compress:   getEnvBool("LOG_COMPRESS", true),
	}
}

// ToConfig converts EnvConfig to the basic Config struct.
// This allows backward compatibility with existing code.
func (e *EnvConfig) ToConfig() *Config {
	return &Config{
		Level:       e.Level,
		Format:      e.Format,
		Output:      e.Output,
		ServiceName: e.ServiceName,
	}
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvBool gets a boolean environment variable with a default value.
func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

// getEnvInt gets an integer environment variable with a default value.
func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}
