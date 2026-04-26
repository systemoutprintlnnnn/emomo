package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(t *testing.T, statusCode int, body any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal response body: %v", err)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(bytes.NewReader(payload)),
		Request: &http.Request{Method: http.MethodPost},
	}
}

func TestJinaEmbeddingProviderEmbedDocumentImageModeUsesImageInput(t *testing.T) {
	t.Parallel()

	var got jinaRequest
	provider := NewJinaEmbeddingProvider(&EmbeddingProviderConfig{
		Model:        "jina-embeddings-v4",
		APIKey:       "test-key",
		BaseURL:      "https://jina.test",
		DocumentMode: embeddingDocumentImage,
		Dimensions:   2048,
	})
	provider.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		return jsonResponse(t, http.StatusOK, jinaResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
			},
		}), nil
	}))

	embedding, err := provider.EmbedDocument(context.Background(), EmbeddingDocument{
		Text:     "desc:happy meme",
		ImageURL: "https://example.com/meme.jpg",
	})
	if err != nil {
		t.Fatalf("EmbedDocument returned error: %v", err)
	}

	if len(embedding) != 3 {
		t.Fatalf("unexpected embedding length: got %d", len(embedding))
	}
	if got.Task != "retrieval.passage" {
		t.Fatalf("unexpected task: got %q", got.Task)
	}
	if got.Model != "jina-embeddings-v4" {
		t.Fatalf("unexpected model: got %q", got.Model)
	}
	if got.Dimensions != 2048 {
		t.Fatalf("unexpected dimensions: got %d", got.Dimensions)
	}
	if len(got.Input) != 1 {
		t.Fatalf("unexpected input length: got %d", len(got.Input))
	}
	if got.Input[0].Image != "https://example.com/meme.jpg" {
		t.Fatalf("unexpected image input: got %q", got.Input[0].Image)
	}
	if got.Input[0].Text != "" {
		t.Fatalf("expected empty text input, got %q", got.Input[0].Text)
	}
}

func TestJinaEmbeddingProviderEmbedDocumentImageModeRequiresImageURL(t *testing.T) {
	t.Parallel()

	provider := NewJinaEmbeddingProvider(&EmbeddingProviderConfig{
		Model:        "jina-embeddings-v4",
		APIKey:       "test-key",
		BaseURL:      "https://api.jina.ai/v1",
		DocumentMode: embeddingDocumentImage,
		Dimensions:   2048,
	})

	if _, err := provider.EmbedDocument(context.Background(), EmbeddingDocument{Text: "desc:happy meme"}); err == nil {
		t.Fatal("expected error when image_url is missing")
	}
}

func TestJinaEmbeddingProviderEmbedQueryUsesTextInput(t *testing.T) {
	t.Parallel()

	var got jinaRequest
	provider := NewJinaEmbeddingProvider(&EmbeddingProviderConfig{
		Model:      "jina-embeddings-v4",
		APIKey:     "test-key",
		BaseURL:    "https://jina.test",
		Dimensions: 2048,
	})
	provider.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		return jsonResponse(t, http.StatusOK, jinaResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.9, 0.8}, Index: 0},
			},
		}), nil
	}))

	if _, err := provider.EmbedQuery(context.Background(), "happy cat"); err != nil {
		t.Fatalf("EmbedQuery returned error: %v", err)
	}

	if got.Task != "retrieval.query" {
		t.Fatalf("unexpected task: got %q", got.Task)
	}
	if len(got.Input) != 1 || got.Input[0].Text != "happy cat" {
		t.Fatalf("unexpected query input: %+v", got.Input)
	}
	if got.Input[0].Image != "" {
		t.Fatalf("expected empty image input, got %q", got.Input[0].Image)
	}
}

func TestOpenAICompatibleEmbeddingProviderEmbedDocumentUsesTextFallback(t *testing.T) {
	t.Parallel()

	var got openAIEmbeddingRequest
	provider := NewOpenAICompatibleEmbeddingProvider(&EmbeddingProviderConfig{
		Model:      "Qwen/Qwen3-Embedding-8B",
		APIKey:     "test-key",
		BaseURL:    "https://openai.test",
		Dimensions: 4096,
	})
	provider.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		return jsonResponse(t, http.StatusOK, openAIEmbeddingResponse{
			Data: []openAIEmbeddingData{
				{Embedding: []float64{1.0, 2.0}, Index: 0},
			},
		}), nil
	}))

	embedding, err := provider.EmbedDocument(context.Background(), EmbeddingDocument{
		Text:     "ocr:你好 desc:开心 tag:猫",
		ImageURL: "https://example.com/ignored.jpg",
	})
	if err != nil {
		t.Fatalf("EmbedDocument returned error: %v", err)
	}

	if len(embedding) != 2 {
		t.Fatalf("unexpected embedding length: got %d", len(embedding))
	}
	if len(got.Input) != 1 || got.Input[0] != "ocr:你好 desc:开心 tag:猫" {
		t.Fatalf("unexpected input payload: %+v", got.Input)
	}
}

func TestSiliconFlowEmbeddingProviderEmbedDocumentImageModeUsesImageContent(t *testing.T) {
	t.Parallel()

	var got siliconFlowEmbeddingRequest
	provider := NewSiliconFlowEmbeddingProvider(&EmbeddingProviderConfig{
		Model:        "Qwen/Qwen3-VL-Embedding-8B",
		APIKey:       "test-key",
		BaseURL:      "https://siliconflow.test/v1",
		DocumentMode: embeddingDocumentImage,
		Dimensions:   1024,
	})
	provider.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", gotAuth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		return jsonResponse(t, http.StatusOK, siliconFlowEmbeddingResponse{
			Data: []siliconFlowEmbeddingData{
				{Embedding: []float64{0.1, 0.2}, Index: 0},
			},
		}), nil
	}))

	embedding, err := provider.EmbedDocument(context.Background(), EmbeddingDocument{
		Text:     "ignored text",
		ImageURL: "https://cdn.example.com/meme.jpg",
	})
	if err != nil {
		t.Fatalf("EmbedDocument returned error: %v", err)
	}

	if len(embedding) != 2 {
		t.Fatalf("unexpected embedding length: got %d", len(embedding))
	}
	if got.Model != "Qwen/Qwen3-VL-Embedding-8B" {
		t.Fatalf("unexpected model: %q", got.Model)
	}
	if got.Dimensions != 1024 {
		t.Fatalf("unexpected dimensions: %d", got.Dimensions)
	}
	if got.EncodingFormat != "float" {
		t.Fatalf("unexpected encoding format: %q", got.EncodingFormat)
	}
	input, ok := got.Input.(map[string]any)
	if !ok {
		t.Fatalf("expected object input, got %T", got.Input)
	}
	if input["image"] != "https://cdn.example.com/meme.jpg" {
		t.Fatalf("unexpected image input: %#v", input)
	}
	if _, exists := input["text"]; exists {
		t.Fatalf("expected image-only input, got %#v", input)
	}
}

func TestSiliconFlowEmbeddingProviderEmbedDocumentTextModeUsesTextContentAndTruncate(t *testing.T) {
	t.Parallel()

	var got siliconFlowEmbeddingRequest
	provider := NewSiliconFlowEmbeddingProvider(&EmbeddingProviderConfig{
		Model:        "Qwen/Qwen3-VL-Embedding-8B",
		APIKey:       "test-key",
		BaseURL:      "https://siliconflow.test/v1",
		DocumentMode: embeddingDocumentText,
		Dimensions:   1024,
	})
	provider.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		return jsonResponse(t, http.StatusOK, siliconFlowEmbeddingResponse{
			Data: []siliconFlowEmbeddingData{
				{Embedding: []float64{0.3, 0.4}, Index: 0},
			},
		}), nil
	}))

	_, err := provider.EmbedDocument(context.Background(), EmbeddingDocument{
		Text: "图中文字：你礼貌吗\n画面描述：表达无语和质问",
	})
	if err != nil {
		t.Fatalf("EmbedDocument returned error: %v", err)
	}

	input, ok := got.Input.(map[string]any)
	if !ok {
		t.Fatalf("expected object input, got %T", got.Input)
	}
	if input["text"] != "图中文字：你礼貌吗\n画面描述：表达无语和质问" {
		t.Fatalf("unexpected text input: %#v", input)
	}
	if got.Truncate != "right" {
		t.Fatalf("unexpected truncate value: %q", got.Truncate)
	}
}
