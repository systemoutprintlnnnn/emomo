package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/source"
	"github.com/timmy/emomo/internal/source/chinesebqb"
	"github.com/timmy/emomo/internal/storage"
)

func main() {
	// Initialize logger first (with defaults)
	appLogger := logger.New(&logger.Config{
		Level:       "info",
		Format:      "json",
		ServiceName: "emomo-ingest",
	})
	logger.SetDefaultLogger(appLogger)

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
		appLogger.WithError(err).Fatal("Failed to load config")
	}

	appLogger.WithFields(logger.Fields{
		"source": *sourceType,
		"limit":  *limit,
		"retry":  *retryPending,
		"force":  *force,
	}).Info("Starting ingestion")

	// Initialize database
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize database")
	}

	// Initialize repositories
	memeRepo := repository.NewMemeRepository(db)
	qdrantRepo, err := repository.NewQdrantRepository(&repository.QdrantConnectionConfig{
		Host:       cfg.Qdrant.Host,
		Port:       cfg.Qdrant.Port,
		Collection: cfg.Qdrant.Collection,
		APIKey:     cfg.Qdrant.APIKey,
		UseTLS:     cfg.Qdrant.UseTLS,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize Qdrant repository")
	}
	defer qdrantRepo.Close()

	// Ensure Qdrant collection exists
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := qdrantRepo.EnsureCollection(ctx); err != nil {
		appLogger.WithError(err).Fatal("Failed to ensure Qdrant collection")
	}

	// Initialize S3-compatible storage (supports R2, S3, etc.)
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

	// Ensure bucket exists
	if err := objectStorage.EnsureBucket(ctx); err != nil {
		appLogger.WithError(err).Fatal("Failed to ensure storage bucket")
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
		appLogger,
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
		appLogger.Info("Received shutdown signal, canceling...")
		cancel()
	}()

	// Run ingestion
	if *retryPending {
		stats, err := ingestService.RetryPending(ctx, *limit)
		if err != nil {
			appLogger.WithError(err).Fatal("Failed to retry pending items")
		}
		appLogger.WithFields(logger.Fields{
			"total":     stats.TotalItems,
			"processed": stats.ProcessedItems,
			"failed":    stats.FailedItems,
		}).Info("Retry completed")
	} else {
		// Get data source
		var src source.Source
		switch *sourceType {
		case "chinesebqb":
			src = chinesebqb.NewAdapter(cfg.Sources.ChineseBQB.RepoPath)
		default:
			appLogger.WithField("source", *sourceType).Fatal("Unknown source type")
		}

		stats, err := ingestService.IngestFromSource(ctx, src, *limit, &service.IngestOptions{
			Force: *force,
		})
		if err != nil {
			appLogger.WithError(err).Fatal("Failed to ingest from source")
		}
		appLogger.WithFields(logger.Fields{
			"total":     stats.TotalItems,
			"processed": stats.ProcessedItems,
			"skipped":   stats.SkippedItems,
			"failed":    stats.FailedItems,
		}).Info("Ingestion completed")
	}
}
