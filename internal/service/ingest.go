package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
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
	"gorm.io/gorm"
)

// IngestService handles the data ingestion pipeline
type IngestService struct {
	memeRepo   *repository.MemeRepository
	qdrantRepo *repository.QdrantRepository
	storage    storage.ObjectStorage
	vlm        *VLMService
	embedding  *EmbeddingService
	logger     *logger.Logger
	workers    int
	batchSize  int
}

// IngestConfig holds configuration for the ingest service
type IngestConfig struct {
	Workers   int
	BatchSize int
}

// NewIngestService creates a new ingest service
func NewIngestService(
	memeRepo *repository.MemeRepository,
	qdrantRepo *repository.QdrantRepository,
	objectStorage storage.ObjectStorage,
	vlm *VLMService,
	embedding *EmbeddingService,
	log *logger.Logger,
	cfg *IngestConfig,
) *IngestService {
	return &IngestService{
		memeRepo:   memeRepo,
		qdrantRepo: qdrantRepo,
		storage:    objectStorage,
		vlm:        vlm,
		embedding:  embedding,
		logger:     log,
		workers:    cfg.Workers,
		batchSize:  cfg.BatchSize,
	}
}

// log returns a logger from context if available, otherwise returns the default logger
func (s *IngestService) log(ctx context.Context) *logger.Logger {
	if l := logger.FromContext(ctx); l != nil {
		return l
	}
	return s.logger
}

// IngestStats holds statistics for an ingestion run
type IngestStats struct {
	TotalItems     int64
	ProcessedItems int64
	SkippedItems   int64
	FailedItems    int64
	StartTime      time.Time
	EndTime        time.Time
}

// IngestOptions holds options for ingestion
type IngestOptions struct {
	Force bool // If true, skip existence checks and force re-process
}

// IngestFromSource ingests memes from a data source
func (s *IngestService) IngestFromSource(ctx context.Context, src source.Source, limit int, opts *IngestOptions) (*IngestStats, error) {
	if opts == nil {
		opts = &IngestOptions{}
	}

	stats := &IngestStats{
		StartTime: time.Now(),
	}

	s.log(ctx).WithFields(logger.Fields{
		"source": src.GetSourceID(),
		"limit":  limit,
		"force":  opts.Force,
	}).Info("Starting ingestion")

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
				s.log(ctx).WithFields(logger.Fields{
					"source_id": result.sourceID,
				}).WithError(result.err).Error("Failed to process item")
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
			s.log(ctx).WithError(err).Error("Failed to fetch batch")
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

	s.log(ctx).WithFields(logger.Fields{
		"total":     stats.TotalItems,
		"processed": stats.ProcessedItems,
		"skipped":   stats.SkippedItems,
		"failed":    stats.FailedItems,
		"duration":  stats.EndTime.Sub(stats.StartTime).String(),
	}).Info("Ingestion completed")

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
		var existingID string

		// Check if already exists by source ID
		existingMeme, err := s.memeRepo.GetBySourceID(ctx, sourceType, item.SourceID)
		if err == nil {
			// Found existing record
			existingID = existingMeme.ID
			if !opts.Force {
				result.skipped = true
				results <- result
				continue
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			// Real error occurred
			result.err = fmt.Errorf("failed to check existence: %w", err)
			results <- result
			continue
		}

		// Process the item
		if err := s.processItem(ctx, sourceType, &item, opts, existingID); err != nil {
			if err == errSkipDuplicate {
				result.skipped = true
			} else {
				result.err = err
			}
		}

		results <- result
	}
}

func (s *IngestService) processItem(ctx context.Context, sourceType string, item *source.MemeItem, opts *IngestOptions, existingID string) error {
	// Read image data
	imageData, err := s.readImage(item)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Calculate MD5 hash
	md5Hash := calculateMD5(imageData)

	// Check for duplicates by MD5 (skip if force is enabled)
	if !opts.Force {
		exists, err := s.memeRepo.ExistsByMD5Hash(ctx, md5Hash)
		if err != nil {
			return fmt.Errorf("failed to check MD5: %w", err)
		}
		if exists {
			return errSkipDuplicate // Return sentinel error to indicate skip
		}
	}

	// Get image dimensions
	width, height, err := getImageDimensions(imageData)
	if err != nil {
		s.log(ctx).WithError(err).Warn("Failed to get image dimensions")
		width, height = 0, 0
	}

	// Determine Meme ID (reuse existing if available, otherwise generate new)
	var memeID string
	if existingID != "" {
		memeID = existingID
	} else {
		memeID = uuid.New().String()
	}

	// Generate VLM description first - most likely to fail (external API)
	// No rollback needed if this fails since nothing has been persisted yet
	description, err := s.vlm.DescribeImage(ctx, imageData, item.Format)
	if err != nil {
		return fmt.Errorf("failed to generate VLM description: %w", err)
	}

	// Generate embedding - also external API, do before any persistence
	embedding, err := s.embedding.Embed(ctx, description)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Now start persistence operations (require rollback on failure)
	// Upload to storage (use MD5 prefix for bucketing, hide source info)
	storageKey := fmt.Sprintf("%s/%s.%s", md5Hash[:2], md5Hash, item.Format)
	contentType := getContentType(item.Format)
	
	// Check if file already exists in storage
	existsInStorage, err := s.storage.Exists(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("failed to check storage existence: %w", err)
	}

	uploaded := false
	if !existsInStorage {
		if err := s.storage.Upload(ctx, storageKey, bytes.NewReader(imageData), int64(len(imageData)), contentType); err != nil {
			return fmt.Errorf("failed to upload to storage: %w", err)
		}
		uploaded = true
	} else {
		s.log(ctx).WithField("storage_key", storageKey).Debug("File already exists in storage, skipping upload")
	}

	// Get storage URL
	storageURL := s.storage.GetURL(storageKey)

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
		QdrantPointID:  memeID, // Use same ID for Qdrant
		VLMDescription: description,
		VLMModel:       s.vlm.GetModel(),
		EmbeddingModel: s.embedding.GetModel(),
		Tags:           item.Tags,
		Category:       item.Category,
		Status:         domain.MemeStatusActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Upsert to Qdrant
	payload := &repository.MemePayload{
		MemeID:         memeID,
		SourceType:     sourceType,
		Category:       item.Category,
		IsAnimated:     item.IsAnimated,
		Tags:           item.Tags,
		VLMDescription: description,
		StorageURL:     storageURL,
	}

	if err := s.qdrantRepo.Upsert(ctx, memeID, embedding, payload); err != nil {
		// Rollback: delete uploaded file ONLY if we uploaded it
		if uploaded {
			if delErr := s.storage.Delete(ctx, storageKey); delErr != nil {
				s.log(ctx).WithFields(logger.Fields{
					"storage_key": storageKey,
				}).WithError(delErr).Error("Failed to rollback storage upload")
			}
		}
		return fmt.Errorf("failed to upsert to Qdrant: %w", err)
	}

	// Save to database (Upsert)
	if err := s.memeRepo.Upsert(ctx, meme); err != nil {
		// Rollback: delete from Qdrant and storage
		if delErr := s.qdrantRepo.Delete(ctx, memeID); delErr != nil {
			s.log(ctx).WithFields(logger.Fields{
				"meme_id": memeID,
			}).WithError(delErr).Error("Failed to rollback Qdrant upsert")
		}
		// Rollback storage ONLY if we uploaded it
		if uploaded {
			if delErr := s.storage.Delete(ctx, storageKey); delErr != nil {
				s.log(ctx).WithFields(logger.Fields{
					"storage_key": storageKey,
				}).WithError(delErr).Error("Failed to rollback storage upload")
			}
		}
		return fmt.Errorf("failed to save to database: %w", err)
	}

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

// RetryPending retries processing for memes with pending status
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
			s.log(ctx).WithError(err).Error("Failed to download from storage")
			stats.FailedItems++
			continue
		}

		imageData, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			s.log(ctx).WithError(err).Error("Failed to read image data")
			stats.FailedItems++
			continue
		}

		// Generate VLM description
		description, err := s.vlm.DescribeImage(ctx, imageData, meme.Format)
		if err != nil {
			s.log(ctx).WithError(err).Warn("Failed to generate VLM description")
			stats.FailedItems++
			continue
		}

		// Generate embedding
		embedding, err := s.embedding.Embed(ctx, description)
		if err != nil {
			s.log(ctx).WithError(err).Warn("Failed to generate embedding")
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
			s.log(ctx).WithError(err).Error("Failed to upsert to Qdrant")
			stats.FailedItems++
			continue
		}

		// Update database
		meme.VLMDescription = description
		meme.Status = domain.MemeStatusActive
		meme.UpdatedAt = time.Now()

		if err := s.memeRepo.Update(ctx, &meme); err != nil {
			s.log(ctx).WithError(err).Error("Failed to update database")
			stats.FailedItems++
			continue
		}

		stats.ProcessedItems++
	}

	stats.EndTime = time.Now()
	return stats, nil
}
