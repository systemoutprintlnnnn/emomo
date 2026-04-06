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
