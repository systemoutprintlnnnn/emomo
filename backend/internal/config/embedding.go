package config

import (
	"fmt"
	"os"
)

// EmbeddingConfig defines configuration for a single embedding provider.
// This is the unified configuration structure used across the application.
type EmbeddingConfig struct {
	Name       string `mapstructure:"name"`         // Unique identifier for this embedding config
	Provider   string `mapstructure:"provider"`     // Provider type: "jina", "modelscope", "openai-compatible"
	Model      string `mapstructure:"model"`        // Model name/ID
	APIKey     string `mapstructure:"api_key"`      // API key (can be set directly or via env var)
	APIKeyEnv  string `mapstructure:"api_key_env"`  // Environment variable name for API key
	BaseURL    string `mapstructure:"base_url"`     // Base URL for OpenAI-compatible APIs
	BaseURLEnv string `mapstructure:"base_url_env"` // Environment variable name for base URL
	Dimensions int    `mapstructure:"dimensions"`   // Embedding vector dimensions
	Collection string `mapstructure:"collection"`   // Qdrant collection name for this embedding
	IsDefault  bool   `mapstructure:"is_default"`   // Whether this is the default embedding config
}

// ResolveEnvVars resolves environment variable references in the configuration.
// If APIKeyEnv or BaseURLEnv are set, their values are loaded from environment.
// Direct values (APIKey, BaseURL) take precedence if already set.
func (c *EmbeddingConfig) ResolveEnvVars() {
	// Resolve API key from environment variable if specified
	if c.APIKeyEnv != "" && c.APIKey == "" {
		if val := os.Getenv(c.APIKeyEnv); val != "" {
			c.APIKey = val
		}
	}

	// Resolve base URL from environment variable if specified
	if c.BaseURLEnv != "" && c.BaseURL == "" {
		if val := os.Getenv(c.BaseURLEnv); val != "" {
			c.BaseURL = val
		}
	}
}

// Validate checks that the embedding configuration has all required fields.
// Returns an error describing the first validation failure, or nil if valid.
func (c *EmbeddingConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("embedding config: name is required")
	}
	if c.Provider == "" {
		return fmt.Errorf("embedding %q: provider is required", c.Name)
	}
	if c.Model == "" {
		return fmt.Errorf("embedding %q: model is required", c.Name)
	}
	if c.Dimensions <= 0 {
		return fmt.Errorf("embedding %q: dimensions must be positive", c.Name)
	}

	// Validate provider is known
	switch c.Provider {
	case "jina", "modelscope", "openai-compatible":
		// Valid providers
	default:
		return fmt.Errorf("embedding %q: unknown provider %q", c.Name, c.Provider)
	}

	return nil
}

// ValidateWithAPIKey validates the configuration including API key requirement.
// Use this when the embedding will actually be used (not just configured).
func (c *EmbeddingConfig) ValidateWithAPIKey() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.APIKey == "" {
		return fmt.Errorf("embedding %q: api_key is required (set directly or via %s)", c.Name, c.APIKeyEnv)
	}
	return nil
}

// GetCollection returns the collection name for this embedding.
// If Collection is empty, returns the provided default collection name.
func (c *EmbeddingConfig) GetCollection(defaultCollection string) string {
	if c.Collection != "" {
		return c.Collection
	}
	return defaultCollection
}

// Clone creates a deep copy of the embedding configuration.
func (c *EmbeddingConfig) Clone() *EmbeddingConfig {
	return &EmbeddingConfig{
		Name:       c.Name,
		Provider:   c.Provider,
		Model:      c.Model,
		APIKey:     c.APIKey,
		APIKeyEnv:  c.APIKeyEnv,
		BaseURL:    c.BaseURL,
		BaseURLEnv: c.BaseURLEnv,
		Dimensions: c.Dimensions,
		Collection: c.Collection,
		IsDefault:  c.IsDefault,
	}
}
