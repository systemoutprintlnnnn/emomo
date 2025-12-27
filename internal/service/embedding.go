package service

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
)

const (
	jinaEndpoint = "https://api.jina.ai/v1/embeddings"
)

// EmbeddingService handles text embedding generation
type EmbeddingService struct {
	client     *resty.Client
	model      string
	dimensions int
}

// EmbeddingConfig holds configuration for embedding service
type EmbeddingConfig struct {
	Provider   string
	Model      string
	APIKey     string
	Dimensions int
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(cfg *EmbeddingConfig) *EmbeddingService {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")

	return &EmbeddingService{
		client:     client,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
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

	req := jinaRequest{
		Model:         s.model,
		Task:          "retrieval.passage", // Optimized for retrieval
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

// EmbedQuery generates an embedding optimized for query/search
func (s *EmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	req := jinaRequest{
		Model:         s.model,
		Task:          "retrieval.query", // Optimized for query
		Dimensions:    s.dimensions,
		Input:         []string{query},
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

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Data[0].Embedding, nil
}
