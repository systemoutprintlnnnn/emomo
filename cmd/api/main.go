package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timmy/emomo/internal/api"
	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/storage"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

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
	ctx := context.Background()
	if err := qdrantRepo.EnsureCollection(ctx); err != nil {
		logger.Fatal("Failed to ensure Qdrant collection", zap.Error(err))
	}

	// Initialize storage
	minioStorage, err := storage.NewMinIOStorage(&storage.MinIOConfig{
		Endpoint:  cfg.MinIO.Endpoint,
		AccessKey: cfg.MinIO.AccessKey,
		SecretKey: cfg.MinIO.SecretKey,
		UseSSL:    cfg.MinIO.UseSSL,
		Bucket:    cfg.MinIO.Bucket,
	})
	if err != nil {
		logger.Fatal("Failed to initialize MinIO storage", zap.Error(err))
	}

	// Ensure bucket exists
	if err := minioStorage.EnsureBucket(ctx); err != nil {
		logger.Fatal("Failed to ensure MinIO bucket", zap.Error(err))
	}

	// Initialize services
	embeddingService := service.NewEmbeddingService(&service.EmbeddingConfig{
		Provider:   cfg.Embedding.Provider,
		Model:      cfg.Embedding.Model,
		APIKey:     cfg.Embedding.APIKey,
		Dimensions: cfg.Embedding.Dimensions,
	})

	// Initialize query expansion service
	queryExpansionService := service.NewQueryExpansionService(&service.QueryExpansionConfig{
		Enabled: cfg.Search.QueryExpansion.Enabled,
		Model:   cfg.Search.QueryExpansion.Model,
		APIKey:  cfg.VLM.APIKey,  // Reuse VLM API key
		BaseURL: cfg.VLM.BaseURL, // Reuse VLM base URL
	})

	if queryExpansionService.IsEnabled() {
		logger.Info("Query expansion enabled",
			zap.String("model", cfg.Search.QueryExpansion.Model),
		)
	}

	searchService := service.NewSearchService(
		memeRepo,
		qdrantRepo,
		embeddingService,
		queryExpansionService,
		logger,
		&service.SearchConfig{
			ScoreThreshold: cfg.Search.ScoreThreshold,
		},
	)

	// Setup router
	router := api.SetupRouter(searchService, cfg.Server.Mode)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting API server",
			zap.Int("port", cfg.Server.Port),
			zap.String("mode", cfg.Server.Mode),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
