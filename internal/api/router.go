package api

import (
	"github.com/gin-gonic/gin"
	"github.com/timmy/emomo/internal/api/handler"
	"github.com/timmy/emomo/internal/api/middleware"
	"github.com/timmy/emomo/internal/config"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/service"
	"github.com/timmy/emomo/internal/source"
)

// SetupRouter configures the Gin router with all routes and middleware.
// Parameters:
//   - searchService: search service used by API handlers.
//   - ingestService: ingest service used by admin handlers.
//   - sources: map of source adapters keyed by name.
//   - cfg: application configuration for server settings.
//   - log: logger instance for middleware.
// Returns:
//   - *gin.Engine: configured Gin router.
func SetupRouter(
	searchService *service.SearchService,
	ingestService *service.IngestService,
	sources map[string]source.Source,
	cfg *config.Config,
	log *logger.Logger,
) *gin.Engine {
	// Set Gin mode
	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// Add middleware
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins:  cfg.Server.CORS.AllowedOrigins,
		AllowAllOrigins: cfg.Server.CORS.AllowAllOrigins,
	}))

	// Create handlers
	healthHandler := handler.NewHealthHandler()
	searchHandler := handler.NewSearchHandler(searchService)
	memeHandler := handler.NewMemeHandler(searchService)
	adminHandler := handler.NewAdminHandler(ingestService, sources, log)

	// Admin page (root)
	r.GET("/", adminHandler.AdminPage)

	// Health check
	r.GET("/health", healthHandler.Health)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Search
		v1.POST("/search", searchHandler.TextSearch)

		// Categories
		v1.GET("/categories", searchHandler.GetCategories)

		// Memes
		v1.GET("/memes", memeHandler.ListMemes)
		v1.GET("/memes/:id", memeHandler.GetMeme)

		// Stats
		v1.GET("/stats", searchHandler.GetStats)

		// Ingest (admin)
		v1.POST("/ingest", adminHandler.TriggerIngest)
		v1.GET("/ingest/status", adminHandler.GetIngestStatus)
	}

	return r
}
