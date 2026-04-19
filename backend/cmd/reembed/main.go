// reembed re-creates Qdrant points for memes that already exist in Postgres,
// using a chosen embedding profile.
//
// Use case: a new embedding model/collection has been added to config.yaml
// (e.g. emomo_jina_v4) but the meme records and their VLM descriptions are
// already in Postgres. This tool walks the memes table, reuses the existing
// VLM description (no Gemini calls), asks the configured embedding provider
// to embed the meme image (Jina v4 image mode = pass the R2 URL), upserts
// the resulting dense + BM25 vector into the target Qdrant collection, and
// writes a meme_vectors row so that subsequent runs skip the meme.
//
// Example:
//
//	go run ./cmd/reembed --embedding jina --limit 5 --workers 4
//	go run ./cmd/reembed --embedding jina --workers 8        # full backfill
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/domain"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/storage"
)

func main() {
	appLogger := logger.New(&logger.Config{
		Level:       "info",
		Format:      "text",
		ServiceName: "emomo-reembed",
	})
	logger.SetDefaultLogger(appLogger)
	defer logger.Sync()

	configPath := flag.String("config", "", "Path to config file (defaults to ./configs/config.yaml)")
	embeddingName := flag.String("embedding", "", "Embedding config name (e.g. 'jina'). Defaults to the config's default embedding")
	limit := flag.Int("limit", 0, "Maximum memes to (re)embed; 0 = no limit")
	workers := flag.Int("workers", 4, "Number of concurrent workers")
	dryRun := flag.Bool("dry-run", false, "Plan only: count memes that would be embedded but do not call any APIs")
	force := flag.Bool("force", false, "Re-embed even if a meme_vectors row already exists for the target collection")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to load config")
	}
	cfg.Database.AutoMigrate = false

	var embeddingCfg *config.EmbeddingConfig
	if *embeddingName != "" {
		embeddingCfg = cfg.GetEmbeddingByName(*embeddingName)
		if embeddingCfg == nil {
			appLogger.WithField("embedding", *embeddingName).Fatal("Unknown embedding configuration name")
		}
	} else {
		embeddingCfg = cfg.GetDefaultEmbedding()
		if embeddingCfg == nil {
			appLogger.Fatal("No embedding configuration found in config.yaml")
		}
	}
	embeddingCfg.ResolveEnvVars()
	if err := embeddingCfg.ValidateWithAPIKey(); err != nil {
		appLogger.WithError(err).Fatal("Invalid embedding configuration")
	}

	collectionName := embeddingCfg.GetCollection(cfg.Qdrant.Collection)
	if collectionName == "" {
		appLogger.Fatal("Embedding has no collection name (set 'collection:' in config.yaml)")
	}

	if embeddingCfg.GetDocumentMode() != "image" {
		appLogger.WithField("document_mode", embeddingCfg.GetDocumentMode()).
			Warn("Selected embedding is NOT in image mode; reembed will fall back to text-based EmbedDocument")
	}

	appLogger.WithFields(logger.Fields{
		"embedding":         embeddingCfg.Name,
		"embedding_model":   embeddingCfg.Model,
		"embedding_dim":     embeddingCfg.Dimensions,
		"qdrant_collection": collectionName,
		"limit":             *limit,
		"workers":           *workers,
		"dry_run":           *dryRun,
		"force":             *force,
	}).Info("Starting reembed")

	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize database")
	}

	memeRepo := repository.NewMemeRepository(db)
	vectorRepo := repository.NewMemeVectorRepository(db)
	descRepo := repository.NewMemeDescriptionRepository(db)

	qdrantRepo, err := repository.NewQdrantRepository(&repository.QdrantConnectionConfig{
		Host:            cfg.Qdrant.Host,
		Port:            cfg.Qdrant.Port,
		Collection:      collectionName,
		APIKey:          cfg.Qdrant.APIKey,
		UseTLS:          cfg.Qdrant.UseTLS,
		VectorDimension: embeddingCfg.Dimensions,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize Qdrant repository")
	}
	defer qdrantRepo.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := qdrantRepo.EnsureCollection(ctx); err != nil {
		appLogger.WithError(err).Fatal("Failed to ensure Qdrant collection")
	}

	storageCfg := cfg.GetStorageConfig()
	objectStorage, err := storage.NewStorage(&storage.S3Config{
		Type:      storage.StorageType(storageCfg.Type),
		Endpoint:  storageCfg.Endpoint,
		AccessKey: storageCfg.AccessKey,
		SecretKey: storageCfg.SecretKey,
		UseSSL:    storageCfg.UseSSL,
		Bucket:    storageCfg.Bucket,
		Region:    storageCfg.Region,
		PublicURL: storageCfg.PublicURL,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize storage")
	}

	embeddingProvider, err := service.NewEmbeddingProvider(&service.EmbeddingProviderConfig{
		Provider:     embeddingCfg.Provider,
		Model:        embeddingCfg.Model,
		APIKey:       embeddingCfg.APIKey,
		BaseURL:      embeddingCfg.BaseURL,
		DocumentMode: embeddingCfg.GetDocumentMode(),
		Dimensions:   embeddingCfg.Dimensions,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to create embedding provider")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		appLogger.Warn("Received shutdown signal, canceling...")
		cancel()
	}()

	w := &worker{
		log:               appLogger,
		memeRepo:          memeRepo,
		vectorRepo:        vectorRepo,
		descRepo:          descRepo,
		qdrantRepo:        qdrantRepo,
		objectStorage:     objectStorage,
		embeddingProvider: embeddingProvider,
		collection:        collectionName,
		dryRun:            *dryRun,
		force:             *force,
	}

	stats, err := w.run(ctx, *limit, *workers)
	if err != nil && !errors.Is(err, context.Canceled) {
		appLogger.WithError(err).Fatal("reembed failed")
	}

	appLogger.WithFields(logger.Fields{
		"scanned":         stats.Scanned,
		"skipped_existed": stats.SkippedExisted,
		"skipped_no_url":  stats.SkippedNoURL,
		"reembedded":      stats.Reembedded,
		"failed":          stats.Failed,
		"collection":      collectionName,
		"model":           embeddingProvider.GetModel(),
	}).Info("Reembed completed")
}

// =============================================================================
// Worker
// =============================================================================

type worker struct {
	log               *logger.Logger
	memeRepo          *repository.MemeRepository
	vectorRepo        *repository.MemeVectorRepository
	descRepo          *repository.MemeDescriptionRepository
	qdrantRepo        *repository.QdrantRepository
	objectStorage     storage.ObjectStorage
	embeddingProvider service.EmbeddingProvider
	collection        string
	dryRun            bool
	force             bool
}

type runStats struct {
	Scanned        int64
	SkippedExisted int64
	SkippedNoURL   int64
	Reembedded     int64
	Failed         int64
}

const pageSize = 200

// run streams memes (status=active) page-by-page and feeds them through a
// fixed pool of workers. Each worker may concurrently call the embedding API
// and upsert into Qdrant — both backends tolerate parallelism, but the user
// can throttle via --workers if they want to respect Jina rate limits.
func (w *worker) run(ctx context.Context, limit, workers int) (runStats, error) {
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan domain.Meme, workers*2)
	stats := runStats{}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for meme := range jobs {
				if ctx.Err() != nil {
					return
				}
				w.processOne(ctx, meme, &stats)
			}
		}(i)
	}

	go func() {
		defer close(jobs)

		offset := 0
		emitted := 0
		for {
			if ctx.Err() != nil {
				return
			}

			memes, err := w.memeRepo.ListByStatus(ctx, domain.MemeStatusActive, pageSize, offset)
			if err != nil {
				w.log.WithError(err).WithField("offset", offset).Error("Failed to list memes; aborting page")
				return
			}
			if len(memes) == 0 {
				return
			}

			for _, meme := range memes {
				if limit > 0 && emitted >= limit {
					return
				}
				select {
				case <-ctx.Done():
					return
				case jobs <- meme:
					emitted++
				}
			}

			if len(memes) < pageSize {
				return
			}
			offset += pageSize
		}
	}()

	wg.Wait()
	return stats, ctx.Err()
}

// processOne handles a single meme. It is called from a worker goroutine, so
// it talks to its own copy of `meme` and only mutates `stats` via atomics.
func (w *worker) processOne(ctx context.Context, meme domain.Meme, stats *runStats) {
	atomic.AddInt64(&stats.Scanned, 1)

	if !w.force {
		exists, err := w.vectorRepo.ExistsByMD5AndCollection(ctx, meme.MD5Hash, w.collection)
		if err != nil {
			atomic.AddInt64(&stats.Failed, 1)
			w.log.WithError(err).WithField("meme_id", meme.ID).Error("Failed to check vector existence")
			return
		}
		if exists {
			atomic.AddInt64(&stats.SkippedExisted, 1)
			return
		}
	}

	if meme.StorageKey == "" {
		atomic.AddInt64(&stats.SkippedNoURL, 1)
		w.log.WithField("meme_id", meme.ID).Warn("Meme has no storage_key; cannot derive image URL")
		return
	}
	imageURL := w.objectStorage.GetURL(meme.StorageKey)
	if imageURL == "" {
		atomic.AddInt64(&stats.SkippedNoURL, 1)
		w.log.WithField("meme_id", meme.ID).Warn("Storage backend returned empty URL")
		return
	}

	// Look up an existing VLM description (any model) so we can populate the
	// Qdrant payload + BM25 sparse vector. Reembed never invokes the VLM.
	desc := w.lookupDescription(ctx, meme.ID)
	vlmDescription := ""
	ocrText := ""
	descriptionID := ""
	if desc != nil {
		vlmDescription = desc.Description
		ocrText = service.NormalizeOCRText(desc.OCRText)
		descriptionID = desc.ID
	}

	if w.dryRun {
		atomic.AddInt64(&stats.Reembedded, 1)
		w.log.WithFields(logger.Fields{
			"meme_id":     meme.ID,
			"md5":         meme.MD5Hash,
			"image_url":   imageURL,
			"has_desc":    desc != nil,
		}).Info("[dry-run] would re-embed meme")
		return
	}

	embedStart := time.Now()
	bm25Text := service.BuildBM25Text(ocrText, service.CompactDescription(vlmDescription), meme.Tags)
	embedding, err := w.embedWithRetry(ctx, service.EmbeddingDocument{
		Text:     bm25Text,
		ImageURL: imageURL,
	}, meme.ID)
	if err != nil {
		atomic.AddInt64(&stats.Failed, 1)
		w.log.WithError(err).WithFields(logger.Fields{
			"meme_id":   meme.ID,
			"image_url": imageURL,
		}).Error("EmbedDocument failed after retries")
		return
	}

	pointID := uuid.New().String()

	payload := &repository.MemePayload{
		MemeID:         meme.ID,
		SourceType:     meme.SourceType,
		Category:       meme.Category,
		IsAnimated:     meme.IsAnimated,
		Tags:           meme.Tags,
		VLMDescription: vlmDescription,
		OCRText:        ocrText,
		StorageURL:     imageURL,
	}

	if err := w.qdrantRepo.UpsertHybrid(ctx, pointID, embedding, bm25Text, payload); err != nil {
		atomic.AddInt64(&stats.Failed, 1)
		w.log.WithError(err).WithField("meme_id", meme.ID).Error("UpsertHybrid failed")
		return
	}

	vectorRecord := &domain.MemeVector{
		ID:             uuid.New().String(),
		MemeID:         meme.ID,
		MD5Hash:        meme.MD5Hash,
		Collection:     w.collection,
		EmbeddingModel: w.embeddingProvider.GetModel(),
		DescriptionID:  descriptionID,
		QdrantPointID:  pointID,
		Status:         domain.MemeVectorStatusActive,
		CreatedAt:      time.Now(),
	}
	if err := w.vectorRepo.Create(ctx, vectorRecord); err != nil {
		// Roll back the Qdrant point so the next run can retry cleanly.
		if delErr := w.qdrantRepo.Delete(ctx, pointID); delErr != nil {
			w.log.WithError(delErr).WithField("point_id", pointID).Error("Failed to roll back Qdrant point after meme_vectors insert failure")
		}
		atomic.AddInt64(&stats.Failed, 1)
		w.log.WithError(err).WithField("meme_id", meme.ID).Error("meme_vectors insert failed")
		return
	}

	atomic.AddInt64(&stats.Reembedded, 1)
	w.log.WithFields(logger.Fields{
		"meme_id":     meme.ID,
		"point_id":    pointID,
		"duration_ms": time.Since(embedStart).Milliseconds(),
	}).Info("Re-embedded meme")
}

// embedWithRetry calls the embedding provider with exponential backoff for
// transient failures (HTTP 429 throttling and 5xx). Jina v4 in particular
// tends to return 429 under modest concurrency on the free tier; retrying
// after a short sleep is enough to recover instead of dropping the meme.
func (w *worker) embedWithRetry(ctx context.Context, doc service.EmbeddingDocument, memeID string) ([]float32, error) {
	const maxAttempts = 5
	backoff := 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		vec, err := w.embeddingProvider.EmbedDocument(ctx, doc)
		if err == nil {
			if attempt > 1 {
				w.log.WithFields(logger.Fields{
					"meme_id": memeID,
					"attempt": attempt,
				}).Info("EmbedDocument succeeded after retry")
			}
			return vec, nil
		}
		lastErr = err
		if !isTransientEmbeddingError(err) || attempt == maxAttempts {
			return nil, err
		}
		wait := backoff
		w.log.WithError(err).WithFields(logger.Fields{
			"meme_id":   memeID,
			"attempt":   attempt,
			"sleep_sec": int(wait.Seconds()),
		}).Warn("Transient EmbedDocument failure; retrying")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return nil, lastErr
}

func isTransientEmbeddingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "429") {
		return true
	}
	if strings.Contains(msg, "status 5") { // 500/502/503/504
		return true
	}
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout") {
		return true
	}
	return false
}

// lookupDescription returns any existing VLM description for the meme. It does
// not error out when none is found — the meme can still be embedded purely
// from its image; we just won't have OCR/desc to populate the payload.
func (w *worker) lookupDescription(ctx context.Context, memeID string) *domain.MemeDescription {
	descs, err := w.descRepo.GetByMemeID(ctx, memeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		w.log.WithError(err).WithField("meme_id", memeID).Warn("Failed to load meme description; continuing without it")
		return nil
	}
	if len(descs) == 0 {
		return nil
	}
	// Prefer the most recently created description if multiple VLM models exist.
	best := descs[0]
	for i := 1; i < len(descs); i++ {
		if descs[i].CreatedAt.After(best.CreatedAt) {
			best = descs[i]
		}
	}
	return &best
}
