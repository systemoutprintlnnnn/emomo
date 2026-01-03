package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/timmy/emomo/internal/service"
)

// SearchHandler handles search-related endpoints.
type SearchHandler struct {
	searchService *service.SearchService
}

// NewSearchHandler creates a new search handler.
// Parameters:
//   - searchService: search service instance.
//
// Returns:
//   - *SearchHandler: initialized handler.
func NewSearchHandler(searchService *service.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// TextSearch handles POST /api/v1/search.
// Parameters:
//   - c: Gin request context.
//
// Returns: none (writes JSON response).
func (h *SearchHandler) TextSearch(c *gin.Context) {
	var req service.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Allow query parameter to override collection
	if collection := c.Query("collection"); collection != "" && req.Collection == "" {
		req.Collection = collection
	}

	result, err := h.searchService.TextSearch(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Search failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetCategories handles GET /api/v1/categories.
// Parameters:
//   - c: Gin request context.
//
// Returns: none (writes JSON response).
func (h *SearchHandler) GetCategories(c *gin.Context) {
	categories, err := h.searchService.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get categories: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
		"total":      len(categories),
	})
}

// GetStats handles GET /api/v1/stats.
// Parameters:
//   - c: Gin request context.
//
// Returns: none (writes JSON response).
func (h *SearchHandler) GetStats(c *gin.Context) {
	stats, err := h.searchService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get stats: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// TextSearchStream handles POST /api/v1/search/stream with SSE.
// Parameters:
//   - c: Gin request context.
//
// Returns: none (writes SSE events).
func (h *SearchHandler) TextSearchStream(c *gin.Context) {
	var req service.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Allow query parameter to override collection
	if collection := c.Query("collection"); collection != "" && req.Collection == "" {
		req.Collection = collection
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	ctx := c.Request.Context()

	// Create progress channel
	progressCh := make(chan service.SearchProgress, 100)

	// Start search in goroutine
	var searchResult *service.SearchResponse
	var searchErr error
	done := make(chan struct{})

	go func() {
		defer close(done)
		searchResult, searchErr = h.searchService.TextSearchWithProgress(ctx, &req, progressCh)
	}()

	// Get the response writer for flushing
	w := c.Writer

	// Stream progress events
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case progress, ok := <-progressCh:
			if !ok {
				// Channel closed, wait for search to complete
				<-done
				// Send final result
				if searchErr != nil {
					errData, _ := json.Marshal(gin.H{
						"stage": "error",
						"error": searchErr.Error(),
					})
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
				} else if searchResult != nil {
					resultData, _ := json.Marshal(gin.H{
						"stage":          "complete",
						"results":        searchResult.Results,
						"total":          searchResult.Total,
						"query":          searchResult.Query,
						"expanded_query": searchResult.ExpandedQuery,
						"collection":     searchResult.Collection,
					})
					fmt.Fprintf(w, "event: complete\ndata: %s\n\n", resultData)
				}
				w.Flush()
				return
			}
			// Write SSE event
			eventType := "progress"
			if progress.Stage == "thinking" {
				eventType = "thinking"
			}
			data, _ := json.Marshal(progress)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
			w.Flush()
		}
	}
}
