package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

// RequestLogger logs incoming HTTP requests using slog
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Skip logging for health check to avoid noise
		if path == "/api/v1/health" {
			return
		}

		end := time.Now()
		latency := end.Sub(start)

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		userAgent := c.Request.UserAgent()

		if raw != "" {
			path = path + "?" + raw
		}

		// Log attributes
		attrs := []any{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status", statusCode),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", userAgent),
		}

		// Add error message if present
		if errorMessage != "" {
			attrs = append(attrs, slog.String("error", errorMessage))
		}

		// Add user ID if authenticated
		if userID, exists := c.Get("userID"); exists {
			attrs = append(attrs, slog.Any("user_id", userID))
		}

		msg := "Incoming request"
		if statusCode >= 500 {
			logger.Log.Error(msg, attrs...)
		} else if statusCode >= 400 {
			logger.Log.Warn(msg, attrs...)
		} else {
			logger.Log.Info(msg, attrs...)
		}
	}
}
