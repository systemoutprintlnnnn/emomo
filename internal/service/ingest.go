package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/timmy/emomo/internal/domain"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/source"
	"github.com/timmy/emomo/internal/storage"
	_ "golang.org/x/image/webp"
)

// IngestService handles the data ingestion pipeline.
type IngestService struct {
	memeRepo   *repository.MemeRepository
	vectorRepo *repository.MemeVectorRepository
	qdrantRepo *repository.QdrantRepository
	storage    storage.ObjectStorage
	vlm        *VLMService
	embedding  EmbeddingProvider
	logger     *logger.Logger
	workers    int
	batchSize  int
	collection string // Target Qdrant collection name
}

// IngestConfig holds configuration for the ingest service.
type IngestConfig struct {
	Workers    int
	BatchSize  int
	Collection string // Target Qdrant collection name
}

// NewIngestService creates a new ingest service.
// Parameters:
//   - memeRepo: repository for meme records.
//   - vectorRepo: repository for meme vectors.
//   - qdrantRepo: Qdrant repository for vector storage.
//   - objectStorage: object storage client for image files.
//   - vlm: vision-language model service for descriptions.
//   - embedding: embedding provider for vector generation.
//   - log: logger instance.
//   - cfg: ingest configuration settings.
// Returns:
//   - *IngestService: initialized ingest service.
func NewIngestService(
	memeRepo *repository.MemeRepository,
	vectorRepo *repository.MemeVectorRepository,
	qdrantRepo *repository.QdrantRepository,
	objectStorage storage.ObjectStorage,
	vlm *VLMService,
	embedding EmbeddingProvider,
	log *logger.Logger,
	cfg *IngestConfig,
) *IngestService {
	return &IngestService{
		memeRepo:   memeRepo,
		vectorRepo: vectorRepo,
		qdrantRepo: qdrantRepo,
		storage:    objectStorage,
		vlm:        vlm,
		embedding:  embedding,
		logger:     log,
		workers:    cfg.Workers,
		batchSize:  cfg.BatchSize,
		collection: cfg.Collection,
	}
}

// log returns a logger from context if available, otherwise returns the default logger
func (s *IngestService) log(ctx context.Context) *logger.Logger {
	if l := logger.FromContext(ctx); l != nil {
		return l
	}
	return s.logger
}

// IngestStats holds statistics for an ingestion run.
type IngestStats struct {
	TotalItems     int64
	ProcessedItems int64
	SkippedItems   int64
	FailedItems    int64
	StartTime      time.Time
	EndTime        time.Time
}

// IngestOptions holds options for ingestion.
type IngestOptions struct {
	Force bool // If true, skip existence checks and force re-process
}

// IngestFromSource ingests memes from a data source.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - src: data source implementation.
//   - limit: maximum number of items to ingest.
//   - opts: ingestion options (nil uses defaults).
// Returns:
//   - *IngestStats: statistics for the ingest run.
//   - error: non-nil if ingestion fails.
func (s *IngestService) IngestFromSource(ctx context.Context, src source.Source, limit int, opts *IngestOptions) (*IngestStats, error) {
	if opts == nil {
		opts = &IngestOptions{}
	}

	// Inject tracing fields into context
	ctx = logger.WithFields(ctx, logger.Fields{
		logger.FieldComponent: "ingest",
		logger.FieldJobID:     uuid.New().String(),
		logger.FieldSource:    src.GetSourceID(),
	})

	stats := &IngestStats{
		StartTime: time.Now(),
	}

	logger.CtxInfo(ctx, "Starting ingestion: source=%s, limit=%d, force=%v",
		src.GetSourceID(), limit, opts.Force)

	// Create work channel and results channel
	itemsChan := make(chan source.MemeItem, s.workers*2)
	resultsChan := make(chan *processResult, s.workers*2)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.worker(ctx, workerID, src.GetSourceID(), itemsChan, resultsChan, opts)
		}(i)
	}

	// Start result collector
	done := make(chan struct{})
	go func() {
		for result := range resultsChan {
			atomic.AddInt64(&stats.ProcessedItems, 1)
			if result.skipped {
				atomic.AddInt64(&stats.SkippedItems, 1)
			} else if result.err != nil {
				atomic.AddInt64(&stats.FailedItems, 1)
				logger.CtxError(ctx, "Failed to process item: source_id=%s, error=%v",
					result.sourceID, result.err)
			}
		}
		close(done)
	}()

	// Fetch items from source
	cursor := ""
	totalFetched := 0
	for {
		if ctx.Err() != nil {
			break
		}

		remaining := limit - totalFetched
		if remaining <= 0 {
			break
		}

		batchLimit := s.batchSize
		if batchLimit > remaining {
			batchLimit = remaining
		}

		items, nextCursor, err := src.FetchBatch(ctx, cursor, batchLimit)
		if err != nil {
			logger.CtxError(ctx, "Failed to fetch batch: error=%v", err)
			break
		}

		if len(items) == 0 {
			break
		}

		atomic.AddInt64(&stats.TotalItems, int64(len(items)))
		totalFetched += len(items)

		for _, item := range items {
			select {
			case itemsChan <- item:
			case <-ctx.Done():
				break
			}
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	// Close items channel and wait for workers
	close(itemsChan)
	wg.Wait()

	// Close results channel and wait for collector
	close(resultsChan)
	<-done

	stats.EndTime = time.Now()
	duration := stats.EndTime.Sub(stats.StartTime)

	logger.With(logger.Fields{
		logger.FieldDurationMs: duration.Milliseconds(),
		logger.FieldCount:      stats.ProcessedItems,
	}).Info(ctx, "Ingestion completed: total=%d, processed=%d, skipped=%d, failed=%d",
		stats.TotalItems, stats.ProcessedItems, stats.SkippedItems, stats.FailedItems)

	return stats, nil
}

type processResult struct {
	sourceID string
	skipped  bool
	err      error
}

// errSkipDuplicate is a sentinel error to indicate MD5 duplicate skip
var errSkipDuplicate = fmt.Errorf("skipped: duplicate MD5")

func (s *IngestService) worker(ctx context.Context, workerID int, sourceType string, items <-chan source.MemeItem, results chan<- *processResult, opts *IngestOptions) {
	for item := range items {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := &processResult{sourceID: item.SourceID}

		// Process the item with the new multi-embedding logic
		if err := s.processItem(ctx, sourceType, &item, opts); err != nil {
			if err == errSkipDuplicate {
				result.skipped = true
			} else {
				result.err = err
			}
		}

		results <- result
	}
}

func (s *IngestService) processItem(ctx context.Context, sourceType string, item *source.MemeItem, opts *IngestOptions) error {
	// Read image data
	imageData, err := s.readImage(item)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Calculate MD5 hash
	md5Hash := calculateMD5(imageData)

	// NEW: Check if vector already exists for this MD5 + Collection combination
	if !opts.Force && s.vectorRepo != nil {
		exists, err := s.vectorRepo.ExistsByMD5AndCollection(ctx, md5Hash, s.collection)
		if err != nil {
			return fmt.Errorf("failed to check vector existence: %w", err)
		}
		if exists {
			return errSkipDuplicate // Already has vector in this collection
		}
	}

	// Check if we have an existing meme record (for resource reuse)
	existingMeme, err := s.memeRepo.GetByMD5Hash(ctx, md5Hash)
	hasExistingMeme := err == nil && existingMeme != nil

	var memeID string
	var storageKey string
	var storageURL string
	var vlmDescription string
	var width, height int
	uploaded := false
	createdNewMeme := false // Track if we created a new meme record for rollback

	// rollbackMeme cleans up the meme record if we created one
	rollbackMeme := func() {
		if createdNewMeme && memeID != "" {
			if delErr := s.memeRepo.Delete(ctx, memeID); delErr != nil {
				logger.CtxError(ctx, "Failed to rollback meme record: meme_id=%s, error=%v", memeID, delErr)
			} else {
				logger.CtxDebug(ctx, "Rolled back meme record: meme_id=%s", memeID)
			}
		}
	}

	// rollbackStorage cleans up the storage upload if we uploaded
	rollbackStorage := func() {
		if uploaded {
			if delErr := s.storage.Delete(ctx, storageKey); delErr != nil {
				logger.CtxError(ctx, "Failed to rollback storage upload: storage_key=%s, error=%v", storageKey, delErr)
			} else {
				logger.CtxDebug(ctx, "Rolled back storage upload: storage_key=%s", storageKey)
			}
		}
	}

	if hasExistingMeme {
		// REUSE existing resources: S3 path and VLM description
		memeID = existingMeme.ID
		storageKey = existingMeme.StorageKey
		storageURL = s.storage.GetURL(storageKey)
		vlmDescription = existingMeme.VLMDescription
		width = existingMeme.Width
		height = existingMeme.Height

		logger.CtxInfo(ctx, "Reusing existing meme record: md5=%s, meme_id=%s, collection=%s",
			md5Hash, memeID, s.collection)
	} else {
		// NEW meme: full processing pipeline
		memeID = uuid.New().String()

		// Get image dimensions
		width, height, err = getImageDimensions(imageData)
		if err != nil {
			logger.CtxWarn(ctx, "Failed to get image dimensions: error=%v", err)
			width, height = 0, 0
		}

		// Generate VLM description - most likely to fail (external API)
		vlmDescription, err = s.vlm.DescribeImage(ctx, imageData, item.Format)
		if err != nil {
			return fmt.Errorf("failed to generate VLM description: %w", err)
		}

		// Upload to storage (use MD5 prefix for bucketing)
		storageKey = fmt.Sprintf("%s/%s.%s", md5Hash[:2], md5Hash, item.Format)
		contentType := getContentType(item.Format)

		// Check if file already exists in storage
		existsInStorage, err := s.storage.Exists(ctx, storageKey)
		if err != nil {
			return fmt.Errorf("failed to check storage existence: %w", err)
		}

		if !existsInStorage {
			if err := s.storage.Upload(ctx, storageKey, bytes.NewReader(imageData), int64(len(imageData)), contentType); err != nil {
				return fmt.Errorf("failed to upload to storage: %w", err)
			}
			uploaded = true
		}

		storageURL = s.storage.GetURL(storageKey)

		// Create meme record
		meme := &domain.Meme{
			ID:             memeID,
			SourceType:     sourceType,
			SourceID:       item.SourceID,
			StorageKey:     storageKey,
			LocalPath:      item.LocalPath,
			Width:          width,
			Height:         height,
			Format:         item.Format,
			IsAnimated:     item.IsAnimated,
			FileSize:       int64(len(imageData)),
			MD5Hash:        md5Hash,
			QdrantPointID:  memeID,
			VLMDescription: vlmDescription,
			VLMModel:       s.vlm.GetModel(),
			EmbeddingModel: s.embedding.GetModel(),
			Tags:           item.Tags,
			Category:       item.Category,
			Status:         domain.MemeStatusActive,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// Save meme to database first
		if err := s.memeRepo.Upsert(ctx, meme); err != nil {
			// Rollback storage if we uploaded
			rollbackStorage()
			return fmt.Errorf("failed to save meme to database: %w", err)
		}
		createdNewMeme = true // Mark that we created a new meme record
	}

	// Generate embedding for the current embedding model
	embedding, err := s.embedding.Embed(ctx, vlmDescription)
	if err != nil {
		// Rollback: clean up meme record and storage if we created them
		rollbackMeme()
		rollbackStorage()
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Generate a new point ID for this vector (different from meme ID for multi-collection support)
	pointID := uuid.New().String()

	// Upsert to Qdrant
	payload := &repository.MemePayload{
		MemeID:         memeID,
		SourceType:     sourceType,
		Category:       item.Category,
		IsAnimated:     item.IsAnimated,
		Tags:           item.Tags,
		VLMDescription: vlmDescription,
		StorageURL:     storageURL,
	}

	if err := s.qdrantRepo.Upsert(ctx, pointID, embedding, payload); err != nil {
		// Rollback: clean up meme record and storage if we created them
		rollbackMeme()
		rollbackStorage()
		return fmt.Errorf("failed to upsert to Qdrant: %w", err)
	}

	// Create meme_vectors record to track this vector
	if s.vectorRepo != nil {
		vectorRecord := &domain.MemeVector{
			ID:             uuid.New().String(),
			MemeID:         memeID,
			MD5Hash:        md5Hash,
			Collection:     s.collection,
			EmbeddingModel: s.embedding.GetModel(),
			QdrantPointID:  pointID,
			Status:         domain.MemeVectorStatusActive,
			CreatedAt:      time.Now(),
		}

		if err := s.vectorRepo.Create(ctx, vectorRecord); err != nil {
			// Rollback: delete from Qdrant first
			if delErr := s.qdrantRepo.Delete(ctx, pointID); delErr != nil {
				logger.CtxError(ctx, "Failed to rollback Qdrant upsert: point_id=%s, error=%v", pointID, delErr)
			}
			// Then rollback meme and storage
			rollbackMeme()
			rollbackStorage()
			return fmt.Errorf("failed to save vector record: %w", err)
		}
	}

	logger.CtxDebug(ctx, "Successfully processed item: meme_id=%s, point_id=%s, collection=%s, model=%s, reused=%v",
		memeID, pointID, s.collection, s.embedding.GetModel(), hasExistingMeme)

	return nil
}

func (s *IngestService) readImage(item *source.MemeItem) ([]byte, error) {
	if item.LocalPath != "" {
		return os.ReadFile(item.LocalPath)
	}
	// TODO: Implement HTTP download for URL-based sources
	return nil, fmt.Errorf("URL-based sources not implemented yet")
}

func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func getImageDimensions(data []byte) (int, int, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func getContentType(format string) string {
	switch format {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// RetryPending retries processing for memes with pending status.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - limit: maximum number of pending memes to retry.
// Returns:
//   - *IngestStats: statistics for the retry run.
//   - error: non-nil if the retry processing fails.
func (s *IngestService) RetryPending(ctx context.Context, limit int) (*IngestStats, error) {
	stats := &IngestStats{
		StartTime: time.Now(),
	}

	memes, err := s.memeRepo.ListByStatus(ctx, domain.MemeStatusPending, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending memes: %w", err)
	}

	stats.TotalItems = int64(len(memes))

	for _, meme := range memes {
		select {
		case <-ctx.Done():
			break
		default:
		}

		// Download from storage
		reader, err := s.storage.Download(ctx, meme.StorageKey)
		if err != nil {
			logger.CtxError(ctx, "Failed to download from storage: error=%v", err)
			stats.FailedItems++
			continue
		}

		imageData, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			logger.CtxError(ctx, "Failed to read image data: error=%v", err)
			stats.FailedItems++
			continue
		}

		// Generate VLM description
		description, err := s.vlm.DescribeImage(ctx, imageData, meme.Format)
		if err != nil {
			logger.CtxWarn(ctx, "Failed to generate VLM description: error=%v", err)
			stats.FailedItems++
			continue
		}

		// Generate embedding
		embedding, err := s.embedding.Embed(ctx, description)
		if err != nil {
			logger.CtxWarn(ctx, "Failed to generate embedding: error=%v", err)
			stats.FailedItems++
			continue
		}

		// Update Qdrant
		payload := &repository.MemePayload{
			MemeID:         meme.ID,
			SourceType:     meme.SourceType,
			Category:       meme.Category,
			IsAnimated:     meme.IsAnimated,
			Tags:           meme.Tags,
			VLMDescription: description,
			StorageURL:     s.storage.GetURL(meme.StorageKey),
		}

		if err := s.qdrantRepo.Upsert(ctx, meme.ID, embedding, payload); err != nil {
			logger.CtxError(ctx, "Failed to upsert to Qdrant: error=%v", err)
			stats.FailedItems++
			continue
		}

		// Update database
		meme.VLMDescription = description
		meme.Status = domain.MemeStatusActive
		meme.UpdatedAt = time.Now()

		if err := s.memeRepo.Update(ctx, &meme); err != nil {
			logger.CtxError(ctx, "Failed to update database: error=%v", err)
			stats.FailedItems++
			continue
		}

		stats.ProcessedItems++
	}

	stats.EndTime = time.Now()
	return stats, nil
}
