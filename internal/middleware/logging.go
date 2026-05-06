package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"ims/internal/requestctx"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "request_id"
)

func Logging() gin.HandlerFunc {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	return func(c *gin.Context) {
		start := time.Now()
		requestID := requestIDFromContext(c)

		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Request = c.Request.WithContext(requestctx.WithRequestID(c.Request.Context(), requestID))

		c.Next()

		logger.Info(
			"http_request",
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.String("latency", time.Since(start).String()),
		)
	}
}

func requestIDFromContext(c *gin.Context) string {
	requestID := c.GetHeader(RequestIDHeader)
	if requestID != "" {
		return requestID
	}

	return newRequestID()
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}

	return hex.EncodeToString(bytes[:])
}
