package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

const (
	jinaEndpoint = "https://api.jina.ai/v1/embeddings"
)

// EmbeddingProvider defines the interface for embedding services.
type EmbeddingProvider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery generates an embedding optimized for query/search.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	// GetModel returns the model name being used.
	GetModel() string
	// GetDimensions returns the embedding dimensions.
	GetDimensions() int
}

// EmbeddingProviderConfig holds configuration for creating an embedding provider.
// This is the minimal configuration needed to instantiate a provider.
type EmbeddingProviderConfig struct {
	Provider   string // Provider type: "jina", "modelscope", "openai-compatible"
	Model      string // Model name/ID
	APIKey     string // API key for authentication
	BaseURL    string // Base URL for OpenAI-compatible APIs
	Dimensions int    // Embedding vector dimensions
}

// NewEmbeddingProvider creates a new embedding provider based on the configuration.
func NewEmbeddingProvider(cfg *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding provider config is nil")
	}

	switch cfg.Provider {
	case "jina":
		return NewJinaEmbeddingProvider(cfg), nil
	case "modelscope", "openai-compatible":
		return NewOpenAICompatibleEmbeddingProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Provider)
	}
}

// =============================================================================
// Jina Embedding Provider
// =============================================================================

// JinaEmbeddingProvider handles text embedding generation using Jina AI.
type JinaEmbeddingProvider struct {
	client     *resty.Client
	model      string
	dimensions int
}

// Jina API request/response structures
type jinaRequest struct {
	Model         string   `json:"model"`
	Task          string   `json:"task,omitempty"`
	Dimensions    int      `json:"dimensions,omitempty"`
	Input         []string `json:"input"`
	EmbeddingType string   `json:"embedding_type,omitempty"`
}

type jinaResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Detail string `json:"detail,omitempty"`
}

// NewJinaEmbeddingProvider creates a new Jina embedding provider.
func NewJinaEmbeddingProvider(cfg *EmbeddingProviderConfig) *JinaEmbeddingProvider {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")

	return &JinaEmbeddingProvider{
		client:     client,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
	}
}

// GetModel returns the model name being used.
func (p *JinaEmbeddingProvider) GetModel() string {
	return p.model
}

// GetDimensions returns the embedding dimensions.
func (p *JinaEmbeddingProvider) GetDimensions() int {
	return p.dimensions
}

// Embed generates an embedding for a single text.
func (p *JinaEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *JinaEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	req := jinaRequest{
		Model:         p.model,
		Task:          "retrieval.passage", // Optimized for retrieval
		Dimensions:    p.dimensions,
		Input:         texts,
		EmbeddingType: "float",
	}

	var resp jinaResponse
	httpResp, err := p.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(jinaEndpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to call Jina API: %w", err)
	}

	if httpResp.StatusCode() != 200 {
		if resp.Detail != "" {
			return nil, fmt.Errorf("Jina API error: %s", resp.Detail)
		}
		return nil, fmt.Errorf("Jina API error: status %d", httpResp.StatusCode())
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("unexpected number of embeddings: got %d, expected %d", len(resp.Data), len(texts))
	}

	// Sort by index to ensure correct order
	embeddings := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

// EmbedQuery generates an embedding optimized for query/search.
func (p *JinaEmbeddingProvider) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	req := jinaRequest{
		Model:         p.model,
		Task:          "retrieval.query", // Optimized for query
		Dimensions:    p.dimensions,
		Input:         []string{query},
		EmbeddingType: "float",
	}

	var resp jinaResponse
	httpResp, err := p.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(jinaEndpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to call Jina API: %w", err)
	}

	if httpResp.StatusCode() != 200 {
		if resp.Detail != "" {
			return nil, fmt.Errorf("Jina API error: %s", resp.Detail)
		}
		return nil, fmt.Errorf("Jina API error: status %d", httpResp.StatusCode())
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Data[0].Embedding, nil
}

// =============================================================================
// OpenAI-Compatible Embedding Provider (ModelScope, etc.)
// =============================================================================

// OpenAICompatibleEmbeddingProvider handles text embedding generation using OpenAI-compatible APIs.
// This supports ModelScope, OpenAI, and other compatible providers.
type OpenAICompatibleEmbeddingProvider struct {
	client     *resty.Client
	model      string
	dimensions int
	baseURL    string
}

// OpenAI-compatible API request/response structures
type openAIEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

type openAIEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type openAIEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []openAIEmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewOpenAICompatibleEmbeddingProvider creates a new OpenAI-compatible embedding provider.
func NewOpenAICompatibleEmbeddingProvider(cfg *EmbeddingProviderConfig) *OpenAICompatibleEmbeddingProvider {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAICompatibleEmbeddingProvider{
		client:     client,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		baseURL:    baseURL,
	}
}

// GetModel returns the model name being used.
func (p *OpenAICompatibleEmbeddingProvider) GetModel() string {
	return p.model
}

// GetDimensions returns the embedding dimensions.
func (p *OpenAICompatibleEmbeddingProvider) GetDimensions() int {
	return p.dimensions
}

// Embed generates an embedding for a single text.
func (p *OpenAICompatibleEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *OpenAICompatibleEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	req := openAIEmbeddingRequest{
		Model:          p.model,
		Input:          texts,
		EncodingFormat: "float",
	}

	// Only include dimensions if specified (some models don't support this parameter)
	if p.dimensions > 0 {
		req.Dimensions = p.dimensions
	}

	endpoint := p.baseURL + "/embeddings"

	httpResp, err := p.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}

	if httpResp.StatusCode() != 200 {
		return nil, fmt.Errorf("embedding API error: status %d, body: %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	// Parse response body manually to avoid resty deserialization issues
	var resp openAIEmbeddingResponse
	if err := json.Unmarshal(httpResp.Body(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w, body: %s", err, string(httpResp.Body()))
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s (type: %s)", resp.Error.Message, resp.Error.Type)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("unexpected number of embeddings: got %d, expected %d, response body: %s", len(resp.Data), len(texts), string(httpResp.Body()))
	}

	// Sort by index to ensure correct order and convert float64 to float32
	embeddings := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index < len(embeddings) {
			// Convert []float64 to []float32
			embedding32 := make([]float32, len(item.Embedding))
			for i, v := range item.Embedding {
				embedding32[i] = float32(v)
			}
			embeddings[item.Index] = embedding32
		}
	}

	return embeddings, nil
}

// EmbedQuery generates an embedding optimized for query/search.
// Note: OpenAI-compatible APIs don't have a separate query mode, so this
// calls the regular embedding endpoint.
func (p *OpenAICompatibleEmbeddingProvider) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return p.Embed(ctx, query)
}
