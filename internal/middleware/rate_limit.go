package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ims/internal/ratelimit"
)

func RateLimit(limiter ratelimit.RateLimiter, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := name + ":" + c.ClientIP()

		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "rate limiter unavailable"})
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}
