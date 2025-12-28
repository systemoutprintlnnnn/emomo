package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/timmy/emomo/internal/logger"
)

// LoggerMiddleware returns a Gin middleware that:
// 1. Injects a request-scoped logger into context with common fields
// 2. Logs request/response details in JSON format
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Generate request ID
		requestID := uuid.New().String()

		// Create request-scoped logger with common fields
		reqLogger := log.WithFields(logger.Fields{
			"request_id": requestID,
			"path":       path,
			"method":     c.Request.Method,
			"client_ip":  c.ClientIP(),
		})

		// Inject logger into request context
		ctx := reqLogger.WithContext(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)

		// Also store in Gin's context for convenience
		c.Set("logger", reqLogger)

		// Add request ID to response headers
		c.Header("X-Request-ID", requestID)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()

		// Build full path with query
		fullPath := path
		if query != "" {
			fullPath = path + "?" + query
		}

		// Log request completion
		reqLogger.WithFields(logger.Fields{
			"status":     status,
			"latency_ms": latency.Milliseconds(),
			"size":       c.Writer.Size(),
			"full_path":  fullPath,
		}).Info("Request completed")
	}
}

// GetLogger extracts logger from Gin context
// Falls back to FromContext if not in Gin context
func GetLogger(c *gin.Context) *logger.Logger {
	if l, exists := c.Get("logger"); exists {
		if log, ok := l.(*logger.Logger); ok {
			return log
		}
	}
	return logger.FromContext(c.Request.Context())
}
