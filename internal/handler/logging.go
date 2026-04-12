package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogger returns a Gin middleware that emits a structured log entry
// for every request once it has been handled.
func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		log.Debug("http request started",
			"method", c.Request.Method,
			"path", path,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"content_length", c.Request.ContentLength,
		)

		c.Next()

		log.Info("http request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"response_size", c.Writer.Size(),
		)
	}
}
