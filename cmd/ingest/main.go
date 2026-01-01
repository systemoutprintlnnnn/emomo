package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/source"
	"github.com/timmy/emomo/internal/source/chinesebqb"
	"github.com/timmy/emomo/internal/source/staging"
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
	embeddingName := flag.String("embedding", "", "Embedding config name (e.g., 'jina', 'qwen3'). If empty, uses default")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to load config")
	}

	// Determine embedding configuration
	var embeddingCfg *config.EmbeddingConfig
	if *embeddingName != "" {
		embeddingCfg = cfg.GetEmbeddingByName(*embeddingName)
		if embeddingCfg == nil {
			appLogger.WithField("embedding", *embeddingName).Fatal("Unknown embedding configuration name")
		}
	} else {
		embeddingCfg = cfg.GetDefaultEmbedding()
		if embeddingCfg == nil {
			appLogger.Fatal("No embedding configuration found")
		}
	}

	// Resolve environment variables for the selected embedding
	embeddingCfg.ResolveEnvVars()

	// Validate embedding configuration
	if err := embeddingCfg.ValidateWithAPIKey(); err != nil {
		appLogger.WithError(err).Fatal("Invalid embedding configuration")
	}

	// Get collection name
	collectionName := embeddingCfg.GetCollection(cfg.Qdrant.Collection)

	appLogger.WithFields(logger.Fields{
		"source":            *sourceType,
		"limit":             *limit,
		"retry":             *retryPending,
		"force":             *force,
		"embedding":         embeddingCfg.Name,
		"embedding_model":   embeddingCfg.Model,
		"embedding_dim":     embeddingCfg.Dimensions,
		"qdrant_collection": collectionName,
	}).Info("Starting ingestion")

	// Initialize database
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize database")
	}

	// Initialize repositories
	memeRepo := repository.NewMemeRepository(db)
	vectorRepo := repository.NewMemeVectorRepository(db)

	// Initialize Qdrant repository
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

	// Ensure Qdrant collection exists
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := qdrantRepo.EnsureCollection(ctx); err != nil {
		appLogger.WithError(err).Fatal("Failed to ensure Qdrant collection")
	}

	// Initialize S3-compatible storage
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

	if err := objectStorage.EnsureBucket(ctx); err != nil {
		appLogger.WithError(err).Fatal("Failed to ensure storage bucket")
	}

	// Initialize VLM service
	vlmService := service.NewVLMService(&service.VLMConfig{
		Provider: cfg.VLM.Provider,
		Model:    cfg.VLM.Model,
		APIKey:   cfg.VLM.APIKey,
		BaseURL:  cfg.VLM.BaseURL,
	})

	// Create embedding provider
	embeddingProvider, err := service.NewEmbeddingProvider(&service.EmbeddingProviderConfig{
		Provider:   embeddingCfg.Provider,
		Model:      embeddingCfg.Model,
		APIKey:     embeddingCfg.APIKey,
		BaseURL:    embeddingCfg.BaseURL,
		Dimensions: embeddingCfg.Dimensions,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to create embedding provider")
	}

	// Initialize ingest service
	ingestService := service.NewIngestService(
		memeRepo,
		vectorRepo,
		qdrantRepo,
		objectStorage,
		vlmService,
		embeddingProvider,
		appLogger,
		&service.IngestConfig{
			Workers:    cfg.Ingest.Workers,
			BatchSize:  cfg.Ingest.BatchSize,
			Collection: collectionName,
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
		switch {
		case *sourceType == "chinesebqb":
			src = chinesebqb.NewAdapter(cfg.Sources.ChineseBQB.RepoPath)
		case strings.HasPrefix(*sourceType, "staging:"):
			sourceID := strings.TrimPrefix(*sourceType, "staging:")
			src = staging.NewAdapter(cfg.Sources.Staging.Path, sourceID)
			appLogger.WithField("staging_source", sourceID).Info("Using staging source")
		default:
			appLogger.WithField("source", *sourceType).Fatal("Unknown source type. Use 'chinesebqb' or 'staging:<source_id>'")
		}

		stats, err := ingestService.IngestFromSource(ctx, src, *limit, &service.IngestOptions{
			Force: *force,
		})
		if err != nil {
			appLogger.WithError(err).Fatal("Failed to ingest from source")
		}
		appLogger.WithFields(logger.Fields{
			"total":      stats.TotalItems,
			"processed":  stats.ProcessedItems,
			"skipped":    stats.SkippedItems,
			"failed":     stats.FailedItems,
			"collection": collectionName,
			"model":      embeddingProvider.GetModel(),
		}).Info("Ingestion completed")
	}
}
