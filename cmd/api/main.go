package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timmy/emomo/internal/api"
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
		ServiceName: "emomo-api",
	})
	logger.SetDefaultLogger(appLogger)

	// Load configuration
	// Support CONFIG_PATH environment variable for production deployments
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.Load(configPath)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to load config")
	}

	// Initialize database
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize database")
	}

	// Initialize repositories
	memeRepo := repository.NewMemeRepository(db)
	vectorRepo := repository.NewMemeVectorRepository(db)

	// Initialize default Qdrant repository
	defaultCollection := cfg.GetCollectionForEmbedding("")
	qdrantRepo, err := repository.NewQdrantRepository(&repository.QdrantConnectionConfig{
		Host:            cfg.Qdrant.Host,
		Port:            cfg.Qdrant.Port,
		Collection:      defaultCollection,
		APIKey:          cfg.Qdrant.APIKey,
		UseTLS:          cfg.Qdrant.UseTLS,
		VectorDimension: cfg.Embedding.Dimensions,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize Qdrant repository")
	}
	defer qdrantRepo.Close()

	// Ensure default Qdrant collection exists
	ctx := context.Background()
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

	// Initialize default embedding provider
	defaultEmbedding, err := service.NewEmbeddingProvider(&service.EmbeddingConfig{
		Provider:   cfg.Embedding.Provider,
		Model:      cfg.Embedding.Model,
		APIKey:     cfg.Embedding.APIKey,
		BaseURL:    cfg.Embedding.BaseURL,
		Dimensions: cfg.Embedding.Dimensions,
	})
	if err != nil {
		appLogger.WithError(err).Fatal("Failed to initialize default embedding provider")
	}

	// Initialize query expansion service
	queryExpansionService := service.NewQueryExpansionService(&service.QueryExpansionConfig{
		Enabled: cfg.Search.QueryExpansion.Enabled,
		Model:   cfg.Search.QueryExpansion.Model,
		APIKey:  cfg.VLM.APIKey,  // Reuse VLM API key
		BaseURL: cfg.VLM.BaseURL, // Reuse VLM base URL
	})

	if queryExpansionService.IsEnabled() {
		appLogger.WithFields(logger.Fields{
			"model": cfg.Search.QueryExpansion.Model,
		}).Info("Query expansion enabled")
	}

	// Create search service with default collection
	searchService := service.NewSearchService(
		memeRepo,
		qdrantRepo,
		defaultEmbedding,
		queryExpansionService,
		objectStorage,
		appLogger,
		&service.SearchConfig{
			ScoreThreshold:    cfg.Search.ScoreThreshold,
			DefaultCollection: defaultCollection,
		},
	)

	// Track additional qdrant repos for cleanup
	var additionalQdrantRepos []*repository.QdrantRepository

	// Initialize additional embedding providers and register collections
	for name, embCfg := range cfg.Embeddings {
		// Skip if it's the same as default
		collectionName := cfg.GetCollectionForEmbedding(name)
		if collectionName == defaultCollection {
			continue
		}

		// Check if API key is available
		if embCfg.APIKey == "" {
			appLogger.WithFields(logger.Fields{
				"name":       name,
				"collection": collectionName,
			}).Warn("Skipping embedding config: no API key configured")
			continue
		}

		// Create embedding provider
		embeddingProvider, err := service.NewEmbeddingProvider(&service.EmbeddingConfig{
			Provider:   embCfg.Provider,
			Model:      embCfg.Model,
			APIKey:     embCfg.APIKey,
			BaseURL:    embCfg.BaseURL,
			Dimensions: embCfg.Dimensions,
		})
		if err != nil {
			appLogger.WithFields(logger.Fields{
				"name":  name,
				"error": err.Error(),
			}).Warn("Failed to create embedding provider, skipping")
			continue
		}

		// Create Qdrant repository for this collection
		additionalQdrantRepo, err := repository.NewQdrantRepository(&repository.QdrantConnectionConfig{
			Host:            cfg.Qdrant.Host,
			Port:            cfg.Qdrant.Port,
			Collection:      collectionName,
			APIKey:          cfg.Qdrant.APIKey,
			UseTLS:          cfg.Qdrant.UseTLS,
			VectorDimension: embCfg.Dimensions,
		})
		if err != nil {
			appLogger.WithFields(logger.Fields{
				"name":       name,
				"collection": collectionName,
				"error":      err.Error(),
			}).Warn("Failed to create Qdrant repository, skipping")
			continue
		}
		additionalQdrantRepos = append(additionalQdrantRepos, additionalQdrantRepo)

		// Ensure collection exists (but don't fail if it doesn't)
		if err := additionalQdrantRepo.EnsureCollection(ctx); err != nil {
			appLogger.WithFields(logger.Fields{
				"name":       name,
				"collection": collectionName,
				"error":      err.Error(),
			}).Warn("Failed to ensure collection, but continuing")
		}

		// Register with search service
		searchService.RegisterCollection(name, additionalQdrantRepo, embeddingProvider)

		appLogger.WithFields(logger.Fields{
			"name":       name,
			"collection": collectionName,
			"provider":   embCfg.Provider,
			"model":      embCfg.Model,
			"dimensions": embCfg.Dimensions,
		}).Info("Registered additional embedding collection")
	}

	// Cleanup additional qdrant repos on exit
	defer func() {
		for _, repo := range additionalQdrantRepos {
			repo.Close()
		}
	}()

	// Initialize VLM service for ingest
	vlmService := service.NewVLMService(&service.VLMConfig{
		Provider: cfg.VLM.Provider,
		Model:    cfg.VLM.Model,
		APIKey:   cfg.VLM.APIKey,
		BaseURL:  cfg.VLM.BaseURL,
	})

	// Initialize ingest service (uses default embedding for web UI ingest)
	ingestService := service.NewIngestService(
		memeRepo,
		vectorRepo,
		qdrantRepo,
		objectStorage,
		vlmService,
		defaultEmbedding,
		appLogger,
		&service.IngestConfig{
			Workers:    cfg.Ingest.Workers,
			BatchSize:  cfg.Ingest.BatchSize,
			Collection: defaultCollection,
		},
	)

	// Initialize data sources
	sources := map[string]source.Source{
		"chinesebqb": chinesebqb.NewAdapter(cfg.Sources.ChineseBQB.RepoPath),
	}

	// Setup router
	router := api.SetupRouter(searchService, ingestService, sources, cfg, appLogger)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		appLogger.WithFields(logger.Fields{
			"port":               cfg.Server.Port,
			"mode":               cfg.Server.Mode,
			"default_collection": defaultCollection,
			"available_collections": searchService.GetAvailableCollections(),
		}).Info("Starting API server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.WithError(err).Fatal("Server forced to shutdown")
	}

	appLogger.Info("Server exited")
}
