package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/source"
	"github.com/timmy/emomo/internal/source/chinesebqb"
	"github.com/timmy/emomo/internal/storage"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	sourceType := flag.String("source", "chinesebqb", "Data source to ingest from")
	limit := flag.Int("limit", 100, "Maximum number of items to ingest")
	retryPending := flag.Bool("retry", false, "Retry pending items instead of ingesting new ones")
	force := flag.Bool("force", false, "Force re-process items, skip duplicate checks")
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting ingestion",
		zap.String("source", *sourceType),
		zap.Int("limit", *limit),
		zap.Bool("retry", *retryPending),
		zap.Bool("force", *force),
	)

	// Initialize database
	db, err := repository.InitDB(cfg.Database.Path)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize repositories
	memeRepo := repository.NewMemeRepository(db)
	qdrantRepo, err := repository.NewQdrantRepository(
		cfg.Qdrant.Host,
		cfg.Qdrant.Port,
		cfg.Qdrant.Collection,
	)
	if err != nil {
		logger.Fatal("Failed to initialize Qdrant repository", zap.Error(err))
	}
	defer qdrantRepo.Close()

	// Ensure Qdrant collection exists
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := qdrantRepo.EnsureCollection(ctx); err != nil {
		logger.Fatal("Failed to ensure Qdrant collection", zap.Error(err))
	}

	// Initialize storage (supports MinIO, R2, S3)
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
		logger.Fatal("Failed to initialize storage", zap.Error(err))
	}

	// Ensure bucket exists
	if err := objectStorage.EnsureBucket(ctx); err != nil {
		logger.Fatal("Failed to ensure storage bucket", zap.Error(err))
	}

	// Initialize services
	vlmService := service.NewVLMService(&service.VLMConfig{
		Provider: cfg.VLM.Provider,
		Model:    cfg.VLM.Model,
		APIKey:   cfg.VLM.APIKey,
		BaseURL:  cfg.VLM.BaseURL,
	})

	embeddingService := service.NewEmbeddingService(&service.EmbeddingConfig{
		Provider:   cfg.Embedding.Provider,
		Model:      cfg.Embedding.Model,
		APIKey:     cfg.Embedding.APIKey,
		Dimensions: cfg.Embedding.Dimensions,
	})

	ingestService := service.NewIngestService(
		memeRepo,
		qdrantRepo,
		objectStorage,
		vlmService,
		embeddingService,
		logger,
		&service.IngestConfig{
			Workers:   cfg.Ingest.Workers,
			BatchSize: cfg.Ingest.BatchSize,
		},
	)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, canceling...")
		cancel()
	}()

	// Run ingestion
	if *retryPending {
		stats, err := ingestService.RetryPending(ctx, *limit)
		if err != nil {
			logger.Fatal("Failed to retry pending items", zap.Error(err))
		}
		logger.Info("Retry completed",
			zap.Int64("total", stats.TotalItems),
			zap.Int64("processed", stats.ProcessedItems),
			zap.Int64("failed", stats.FailedItems),
		)
	} else {
		// Get data source
		var src source.Source
		switch *sourceType {
		case "chinesebqb":
			src = chinesebqb.NewAdapter(cfg.Sources.ChineseBQB.RepoPath)
		default:
			logger.Fatal("Unknown source type", zap.String("source", *sourceType))
		}

		stats, err := ingestService.IngestFromSource(ctx, src, *limit, &service.IngestOptions{
			Force: *force,
		})
		if err != nil {
			logger.Fatal("Failed to ingest from source", zap.Error(err))
		}
		logger.Info("Ingestion completed",
			zap.Int64("total", stats.TotalItems),
			zap.Int64("processed", stats.ProcessedItems),
			zap.Int64("skipped", stats.SkippedItems),
			zap.Int64("failed", stats.FailedItems),
		)
	}
}
