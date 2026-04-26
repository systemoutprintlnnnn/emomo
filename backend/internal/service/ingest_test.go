package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/timmy/emomo/internal/domain"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/source"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIsSupportedStaticImageFormatRejectsGIF(t *testing.T) {
	t.Parallel()

	if isSupportedStaticImageFormat("gif") {
		t.Fatal("isSupportedStaticImageFormat(\"gif\") = true, want false")
	}
}

func TestProcessItemRejectsGIFMagicBytes(t *testing.T) {
	t.Parallel()

	imagePath := filepath.Join(t.TempDir(), "looks-static.jpg")
	if err := os.WriteFile(imagePath, []byte("GIF89a-static"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	service := &IngestService{}
	err := service.processItem(context.Background(), "test", &source.MemeItem{
		SourceID:  "deceptive-gif",
		LocalPath: imagePath,
		Format:    "jpeg",
	}, &IngestOptions{})

	if !errors.Is(err, errSkipUnsupportedImageFormat) {
		t.Fatalf("processItem() error = %v, want errSkipUnsupportedImageFormat", err)
	}
}

func TestProcessItemRollsBackNewMemeWhenVectorWriteFails(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.Meme{}, &domain.MemeVector{}, &domain.MemeDescription{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	imagePath := filepath.Join(t.TempDir(), "meme.png")
	if err := os.WriteFile(imagePath, testPNG1x1, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := newMemoryObjectStorage()
	vlm := NewVLMService(&VLMConfig{
		Model:   "test-vlm",
		APIKey:  "test-key",
		BaseURL: "https://vlm.test/v1",
	})
	vlm.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusOK, openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "开心质问的表情包"}},
			},
		}), nil
	}))

	ingest := NewIngestService(
		repository.NewMemeRepository(db),
		repository.NewMemeVectorRepository(db),
		repository.NewMemeDescriptionRepository(db),
		nil,
		store,
		vlm,
		nil,
		nil,
		&IngestConfig{
			Workers:    1,
			BatchSize:  1,
			Collection: "broken_collection",
			VectorIndexes: []IngestVectorIndex{
				{
					VectorType: domain.MemeVectorTypeImage,
					Collection: "broken_collection",
					Embedding:  fixedEmbeddingProvider{},
				},
			},
		},
	)

	err = ingest.processItem(context.Background(), "test", &source.MemeItem{
		SourceID:  "new-meme",
		LocalPath: imagePath,
		Format:    "png",
		Category:  "reaction",
		Tags:      []string{"happy"},
	}, &IngestOptions{})
	if err == nil {
		t.Fatal("processItem() error = nil, want vector write failure")
	}

	var memeCount int64
	if err := db.Model(&domain.Meme{}).Count(&memeCount).Error; err != nil {
		t.Fatalf("count memes: %v", err)
	}
	if memeCount != 0 {
		t.Fatalf("meme count after rollback = %d, want 0", memeCount)
	}

	var descriptionCount int64
	if err := db.Model(&domain.MemeDescription{}).Count(&descriptionCount).Error; err != nil {
		t.Fatalf("count descriptions: %v", err)
	}
	if descriptionCount != 0 {
		t.Fatalf("description count after rollback = %d, want 0", descriptionCount)
	}

	if len(store.objects) != 0 {
		t.Fatalf("storage objects after rollback = %d, want 0", len(store.objects))
	}
	if store.deleteCount != 1 {
		t.Fatalf("storage delete count = %d, want 1", store.deleteCount)
	}
}

func TestNewIngestServiceFallbackIndexUsesConfiguredVectorType(t *testing.T) {
	t.Parallel()

	ingest := NewIngestService(
		nil,
		nil,
		nil,
		&repository.QdrantRepository{},
		nil,
		nil,
		fixedEmbeddingProvider{},
		nil,
		&IngestConfig{
			Collection: "caption_collection",
			VectorType: domain.MemeVectorTypeCaption,
		},
	)

	if len(ingest.indexes) != 1 {
		t.Fatalf("fallback indexes = %d, want 1", len(ingest.indexes))
	}
	if ingest.indexes[0].VectorType != domain.MemeVectorTypeCaption {
		t.Fatalf("fallback vector type = %q, want %q", ingest.indexes[0].VectorType, domain.MemeVectorTypeCaption)
	}
}

var testPNG1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
	0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

type fixedEmbeddingProvider struct{}

func (fixedEmbeddingProvider) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (fixedEmbeddingProvider) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2}}, nil
}

func (fixedEmbeddingProvider) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (fixedEmbeddingProvider) EmbedDocument(context.Context, EmbeddingDocument) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (fixedEmbeddingProvider) GetModel() string {
	return "fixed-test-embedding"
}

func (fixedEmbeddingProvider) GetDimensions() int {
	return 2
}

type memoryObjectStorage struct {
	objects     map[string][]byte
	deleteCount int
}

func newMemoryObjectStorage() *memoryObjectStorage {
	return &memoryObjectStorage{objects: make(map[string][]byte)}
}

func (s *memoryObjectStorage) EnsureBucket(context.Context) error {
	return nil
}

func (s *memoryObjectStorage) Upload(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = data
	return nil
}

func (s *memoryObjectStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memoryObjectStorage) GetURL(key string) string {
	return "https://storage.test/" + key
}

func (s *memoryObjectStorage) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	s.deleteCount++
	return nil
}

func (s *memoryObjectStorage) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.objects[key]
	return ok, nil
}
