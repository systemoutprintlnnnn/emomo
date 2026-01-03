package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/timmy/emomo/internal/service"
)

// #region agent log
func logDebugMeme(location, message string, data map[string]interface{}, hypothesisId string) {
	const debugLogPath = "/Users/timmy/ProgrammingProjects/emomo/.cursor/debug.log"
	logEntry := map[string]interface{}{
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
		"sessionId":    "debug-session",
		"runId":        "run1",
		"hypothesisId": hypothesisId,
	}
	jsonData, _ := json.Marshal(logEntry)
	f, _ := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, "%s\n", jsonData)
		f.Close()
	}
}
// #endregion

// MemeHandler handles meme-related endpoints.
type MemeHandler struct {
	searchService *service.SearchService
}

// NewMemeHandler creates a new meme handler.
// Parameters:
//   - searchService: search service instance.
// Returns:
//   - *MemeHandler: initialized handler.
func NewMemeHandler(searchService *service.SearchService) *MemeHandler {
	return &MemeHandler{
		searchService: searchService,
	}
}

// ListMemes handles GET /api/v1/memes.
// Parameters:
//   - c: Gin request context.
// Returns: none (writes JSON response).
func (h *MemeHandler) ListMemes(c *gin.Context) {
	// #region agent log
	logDebugMeme("handler/meme.go:ListMemes", "ListMemes called", map[string]interface{}{"origin": c.GetHeader("Origin")}, "D")
	// #endregion
	category := c.Query("category")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// #region agent log
	logDebugMeme("handler/meme.go:ListMemes", "Before ListMemes service call", map[string]interface{}{"category": category, "limit": limit, "offset": offset}, "D")
	// #endregion

	result, err := h.searchService.ListMemes(c.Request.Context(), category, limit, offset)
	if err != nil {
		// #region agent log
		logDebugMeme("handler/meme.go:ListMemes", "ListMemes service error", map[string]interface{}{"error": err.Error()}, "D")
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list memes: " + err.Error(),
		})
		return
	}

	// #region agent log
	logDebugMeme("handler/meme.go:ListMemes", "ListMemes success", map[string]interface{}{"total": result.Total, "resultsCount": len(result.Results)}, "D")
	// #endregion
	c.JSON(http.StatusOK, result)
}

// GetMeme handles GET /api/v1/memes/:id.
// Parameters:
//   - c: Gin request context.
// Returns: none (writes JSON response).
func (h *MemeHandler) GetMeme(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Meme ID is required",
		})
		return
	}

	meme, err := h.searchService.GetMemeByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Meme not found",
		})
		return
	}

	c.JSON(http.StatusOK, meme)
}
