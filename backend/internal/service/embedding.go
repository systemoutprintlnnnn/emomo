package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	jinaDefaultBaseURL       = "https://api.jina.ai/v1"
	siliconFlowDefaultURL    = "https://api.siliconflow.cn/v1"
	embeddingDocumentText    = "text"
	embeddingDocumentImage   = "image"
	maxSiliconFlowImageBytes = 25 << 20
)

// EmbeddingProvider defines the interface for embedding services.
type EmbeddingProvider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery generates an embedding optimized for query/search.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	// EmbedDocument generates an embedding for an ingest document.
	EmbedDocument(ctx context.Context, doc EmbeddingDocument) ([]float32, error)
	// GetModel returns the model name being used.
	GetModel() string
	// GetDimensions returns the embedding dimensions.
	GetDimensions() int
}

// EmbeddingDocument carries the content needed to generate an ingest-time embedding.
// Providers can choose the most suitable representation for the configured document mode.
type EmbeddingDocument struct {
	Text           string
	ImageURL       string
	ImageData      []byte
	ImageMediaType string
	Contents       []EmbeddingContent
}

// EmbeddingContent is a provider-neutral multimodal input item.
type EmbeddingContent struct {
	Text  string
	Image string
}

// EmbeddingProviderConfig holds configuration for creating an embedding provider.
// This is the minimal configuration needed to instantiate a provider.
type EmbeddingProviderConfig struct {
	Provider     string // Provider type: "jina", "modelscope", "openai-compatible", "siliconflow"
	Model        string // Model name/ID
	APIKey       string // API key for authentication
	BaseURL      string // Base URL for provider APIs
	DocumentMode string // Document embedding mode: "text" or "image"
	Dimensions   int    // Embedding vector dimensions
}

// NewEmbeddingProvider creates a new embedding provider based on the configuration.
func NewEmbeddingProvider(cfg *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding provider config is nil")
	}

	switch cfg.Provider {
	case "jina":
		return NewJinaEmbeddingProvider(cfg), nil
	case "siliconflow":
		return NewSiliconFlowEmbeddingProvider(cfg), nil
	case "modelscope", "openai-compatible":
		return NewOpenAICompatibleEmbeddingProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Provider)
	}
}

// =============================================================================
// SiliconFlow Embedding Provider
// =============================================================================

// SiliconFlowEmbeddingProvider handles multimodal embedding generation using SiliconFlow.
type SiliconFlowEmbeddingProvider struct {
	client       *resty.Client
	imageClient  *http.Client
	baseURL      string
	model        string
	documentMode string
	dimensions   int
}

type siliconFlowEmbeddingRequest struct {
	Model          string `json:"model"`
	Input          any    `json:"input"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
	Truncate       string `json:"truncate,omitempty"`
	User           string `json:"user,omitempty"`
}

type siliconFlowEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type siliconFlowEmbeddingResponse struct {
	Object string                     `json:"object"`
	Model  string                     `json:"model"`
	Data   []siliconFlowEmbeddingData `json:"data"`
	Usage  struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewSiliconFlowEmbeddingProvider creates a new SiliconFlow embedding provider.
func NewSiliconFlowEmbeddingProvider(cfg *EmbeddingProviderConfig) *SiliconFlowEmbeddingProvider {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = siliconFlowDefaultURL
	}

	return &SiliconFlowEmbeddingProvider{
		client:       client,
		imageClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		model:        cfg.Model,
		documentMode: normalizeEmbeddingDocumentMode(cfg.DocumentMode),
		dimensions:   cfg.Dimensions,
	}
}

func (p *SiliconFlowEmbeddingProvider) GetModel() string {
	return p.model
}

func (p *SiliconFlowEmbeddingProvider) GetDimensions() int {
	return p.dimensions
}

func (p *SiliconFlowEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.embedOne(ctx, map[string]string{"text": text}, true)
}

func (p *SiliconFlowEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	input := make([]any, 0, len(texts))
	for _, text := range texts {
		input = append(input, map[string]string{"text": text})
	}
	return p.embedMany(ctx, input, true)
}

func (p *SiliconFlowEmbeddingProvider) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return p.embedOne(ctx, map[string]string{"text": query}, true)
}

func (p *SiliconFlowEmbeddingProvider) EmbedDocument(ctx context.Context, doc EmbeddingDocument) ([]float32, error) {
	if len(doc.Contents) > 0 {
		input := make([]any, 0, len(doc.Contents))
		for _, content := range doc.Contents {
			if content.Image != "" {
				imageInput, err := p.imageInput(ctx, EmbeddingDocument{ImageURL: content.Image})
				if err != nil {
					return nil, err
				}
				input = append(input, map[string]string{"image": imageInput})
				continue
			}
			if content.Text != "" {
				input = append(input, map[string]string{"text": content.Text})
			}
		}
		if len(input) == 0 {
			return nil, fmt.Errorf("siliconflow document embedding requires non-empty content")
		}
		return p.embedOne(ctx, input, true)
	}

	switch p.documentMode {
	case embeddingDocumentImage:
		imageInput, err := p.imageInput(ctx, doc)
		if err != nil {
			return nil, err
		}
		return p.embedOne(ctx, map[string]string{"image": imageInput}, false)
	default:
		if strings.TrimSpace(doc.Text) == "" {
			return nil, fmt.Errorf("siliconflow text document embedding requires text")
		}
		return p.embedOne(ctx, map[string]string{"text": doc.Text}, true)
	}
}

func (p *SiliconFlowEmbeddingProvider) embedOne(ctx context.Context, input any, truncate bool) ([]float32, error) {
	embeddings, err := p.embedMany(ctx, input, truncate)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

func (p *SiliconFlowEmbeddingProvider) embedMany(ctx context.Context, input any, truncate bool) ([][]float32, error) {
	req := siliconFlowEmbeddingRequest{
		Model:          p.model,
		Input:          input,
		EncodingFormat: "float",
	}
	if p.dimensions > 0 {
		req.Dimensions = p.dimensions
	}
	if truncate {
		req.Truncate = "right"
	}

	httpResp, err := p.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(p.baseURL + "/embeddings")
	if err != nil {
		return nil, fmt.Errorf("failed to call SiliconFlow embedding API: %w", err)
	}
	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("SiliconFlow embedding API error: status %d, body: %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	var resp siliconFlowEmbeddingResponse
	if err := json.Unmarshal(httpResp.Body(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse SiliconFlow embedding response: %w, body: %s", err, string(httpResp.Body()))
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("SiliconFlow embedding API error: %s (type: %s)", resp.Error.Message, resp.Error.Type)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	embeddings := make([][]float32, len(resp.Data))
	for _, item := range resp.Data {
		if item.Index < 0 || item.Index >= len(embeddings) {
			return nil, fmt.Errorf("embedding response index out of range: %d", item.Index)
		}
		embedding32 := make([]float32, len(item.Embedding))
		for i, v := range item.Embedding {
			embedding32[i] = float32(v)
		}
		embeddings[item.Index] = embedding32
	}

	return embeddings, nil
}

func (p *SiliconFlowEmbeddingProvider) imageInput(ctx context.Context, doc EmbeddingDocument) (string, error) {
	if len(doc.ImageData) > 0 {
		return siliconFlowImageDataURI(doc.ImageData, doc.ImageMediaType, doc.ImageURL)
	}

	imageURL := strings.TrimSpace(doc.ImageURL)
	if imageURL == "" {
		return "", fmt.Errorf("siliconflow image document embedding requires image data or image_url")
	}
	if strings.HasPrefix(imageURL, "data:image/") {
		return imageURL, nil
	}

	parsed, err := url.Parse(imageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("siliconflow image document embedding requires image data, data URI, or HTTP(S) image_url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build image download request: %w", err)
	}

	client := p.imageClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image for SiliconFlow embedding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to download image for SiliconFlow embedding: status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxSiliconFlowImageBytes+1)
	imageData, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("failed to read image for SiliconFlow embedding: %w", err)
	}
	if len(imageData) > maxSiliconFlowImageBytes {
		return "", fmt.Errorf("image for SiliconFlow embedding exceeds %d bytes", maxSiliconFlowImageBytes)
	}

	return siliconFlowImageDataURI(imageData, resp.Header.Get("Content-Type"), imageURL)
}

func siliconFlowImageDataURI(imageData []byte, mediaType, source string) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("siliconflow image document embedding requires non-empty image data")
	}

	mediaType = normalizeImageMediaType(mediaType)
	if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
		mediaType = detectImageMediaType(imageData, source)
	}
	if !strings.HasPrefix(mediaType, "image/") {
		return "", fmt.Errorf("siliconflow image document embedding requires an image media type, got %q", mediaType)
	}

	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(imageData), nil
}

func normalizeImageMediaType(mediaType string) string {
	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return strings.ToLower(mediaType)
	}
	return strings.ToLower(parsed)
}

func detectImageMediaType(imageData []byte, source string) string {
	detected := strings.ToLower(http.DetectContentType(imageData))
	if strings.HasPrefix(detected, "image/") {
		return detected
	}

	switch strings.ToLower(filepath.Ext(source)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return detected
	}
}

// =============================================================================
// Jina Embedding Provider
// =============================================================================

// JinaEmbeddingProvider handles text embedding generation using Jina AI.
type JinaEmbeddingProvider struct {
	client       *resty.Client
	baseURL      string
	model        string
	documentMode string
	dimensions   int
}

// Jina API request/response structures
type jinaInputItem struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type jinaRequest struct {
	Model         string          `json:"model"`
	Task          string          `json:"task,omitempty"`
	Dimensions    int             `json:"dimensions,omitempty"`
	Input         []jinaInputItem `json:"input"`
	EmbeddingType string          `json:"embedding_type,omitempty"`
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

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = jinaDefaultBaseURL
	}

	return &JinaEmbeddingProvider{
		client:       client,
		baseURL:      baseURL,
		model:        cfg.Model,
		documentMode: normalizeEmbeddingDocumentMode(cfg.DocumentMode),
		dimensions:   cfg.Dimensions,
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

func (p *JinaEmbeddingProvider) endpoint(path string) string {
	return p.baseURL + path
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

	input := make([]jinaInputItem, 0, len(texts))
	for _, text := range texts {
		input = append(input, jinaInputItem{Text: text})
	}

	req := jinaRequest{
		Model:         p.model,
		Task:          "retrieval.passage", // Optimized for retrieval
		Dimensions:    p.dimensions,
		Input:         input,
		EmbeddingType: "float",
	}

	resp, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
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

// EmbedDocument generates an embedding optimized for ingestion.
func (p *JinaEmbeddingProvider) EmbedDocument(ctx context.Context, doc EmbeddingDocument) ([]float32, error) {
	switch p.documentMode {
	case embeddingDocumentImage:
		if doc.ImageURL == "" {
			return nil, fmt.Errorf("jina image document embedding requires image_url")
		}

		resp, err := p.doRequest(ctx, jinaRequest{
			Model:         p.model,
			Task:          "retrieval.passage",
			Dimensions:    p.dimensions,
			Input:         []jinaInputItem{{Image: doc.ImageURL}},
			EmbeddingType: "float",
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Data) == 0 {
			return nil, fmt.Errorf("no embedding returned")
		}
		return resp.Data[0].Embedding, nil
	default:
		return p.Embed(ctx, doc.Text)
	}
}

// EmbedQuery generates an embedding optimized for query/search.
func (p *JinaEmbeddingProvider) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	resp, err := p.doRequest(ctx, jinaRequest{
		Model:         p.model,
		Task:          "retrieval.query", // Optimized for query
		Dimensions:    p.dimensions,
		Input:         []jinaInputItem{{Text: query}},
		EmbeddingType: "float",
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Data[0].Embedding, nil
}

func (p *JinaEmbeddingProvider) doRequest(ctx context.Context, req jinaRequest) (*jinaResponse, error) {
	var resp jinaResponse
	httpResp, err := p.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(p.endpoint("/embeddings"))

	if err != nil {
		return nil, fmt.Errorf("failed to call Jina API: %w", err)
	}

	if httpResp.StatusCode() != 200 {
		if resp.Detail != "" {
			return nil, fmt.Errorf("Jina API error: %s", resp.Detail)
		}
		return nil, fmt.Errorf("Jina API error: status %d", httpResp.StatusCode())
	}

	return &resp, nil
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

// EmbedDocument generates an embedding for an ingest document.
// OpenAI-compatible providers remain text-only today, so they embed the textual representation.
func (p *OpenAICompatibleEmbeddingProvider) EmbedDocument(ctx context.Context, doc EmbeddingDocument) ([]float32, error) {
	return p.Embed(ctx, doc.Text)
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

func normalizeEmbeddingDocumentMode(mode string) string {
	switch mode {
	case embeddingDocumentImage:
		return embeddingDocumentImage
	default:
		return embeddingDocumentText
	}
}
