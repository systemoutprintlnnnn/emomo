package handler

import (
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
