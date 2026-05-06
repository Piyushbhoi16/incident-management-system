package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoggingAddsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Logging())
	router.GET("/ping", func(c *gin.Context) {
		value, exists := c.Get(RequestIDKey)
		if !exists || value == "" {
			t.Fatal("expected request id in gin context")
		}

		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get(RequestIDHeader) == "" {
		t.Fatal("expected response request id header")
	}
}

func TestLoggingPropagatesIncomingRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Logging())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Header().Get(RequestIDHeader) != "req-123" {
		t.Fatalf("expected propagated request id %q, got %q", "req-123", rec.Header().Get(RequestIDHeader))
	}
}
