package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		log.WithFields(log.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"latency_ms": time.Since(start).Milliseconds(),
			"client_ip":  c.ClientIP(),
			"request_id": c.GetHeader("X-Request-ID"),
		}).Info("request completed")
	}
}
