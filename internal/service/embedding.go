package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

const (
	jinaEndpoint             = "https://api.jina.ai/v1/embeddings"
	modelscopeDefaultBaseURL = "https://api-inference.modelscope.cn/v1"
)

// EmbeddingService handles text embedding generation
type EmbeddingService struct {
	client     *resty.Client
	provider   string
	model      string
	dimensions int
	endpoint   string
}

// EmbeddingConfig holds configuration for embedding service
type EmbeddingConfig struct {
	Provider   string
	Model      string
	APIKey     string
	Dimensions int
	BaseURL    string
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(cfg *EmbeddingConfig) *EmbeddingService {
	client := resty.New()
	if cfg.APIKey != "" {
		client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	}
	client.SetHeader("Content-Type", "application/json")

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "jina"
	}

	endpoint := jinaEndpoint
	if provider == "modelscope" {
		baseURL := strings.TrimRight(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = modelscopeDefaultBaseURL
		}
		endpoint = baseURL + "/embeddings"
	}

	return &EmbeddingService{
		client:     client,
		provider:   provider,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		endpoint:   endpoint,
	}
}

// GetModel returns the model name being used
func (s *EmbeddingService) GetModel() string {
	return s.model
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

type openAIEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
	} `json:"error,omitempty"`
}

// Embed generates an embedding for a single text
func (s *EmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	switch s.provider {
	case "modelscope":
		return s.embedOpenAI(ctx, texts)
	case "jina":
		return s.embedJina(ctx, texts, "retrieval.passage")
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", s.provider)
	}
}

func (s *EmbeddingService) embedJina(ctx context.Context, texts []string, task string) ([][]float32, error) {
	req := jinaRequest{
		Model:         s.model,
		Task:          task, // Optimized for retrieval
		Dimensions:    s.dimensions,
		Input:         texts,
		EmbeddingType: "float",
	}

	var resp jinaResponse
	httpResp, err := s.client.R().
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

func (s *EmbeddingService) embedOpenAI(ctx context.Context, texts []string) ([][]float32, error) {
	req := openAIEmbeddingRequest{
		Model:          s.model,
		Input:          texts,
		EncodingFormat: "float",
	}

	var resp openAIEmbeddingResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}

	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		errorMsg := fmt.Sprintf("HTTP %d", httpResp.StatusCode())
		if resp.Error != nil && resp.Error.Message != "" {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), resp.Error.Message)
		} else if len(httpResp.Body()) > 0 {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), string(httpResp.Body()))
		}
		return nil, fmt.Errorf("embedding API returned error: %s", errorMsg)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", resp.Error.Message)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("unexpected number of embeddings: got %d, expected %d", len(resp.Data), len(texts))
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

// EmbedQuery generates an embedding optimized for query/search
func (s *EmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	var embeddings [][]float32
	var err error

	switch s.provider {
	case "modelscope":
		embeddings, err = s.embedOpenAI(ctx, []string{query})
	case "jina":
		embeddings, err = s.embedJina(ctx, []string{query}, "retrieval.query")
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", s.provider)
	}

	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}
