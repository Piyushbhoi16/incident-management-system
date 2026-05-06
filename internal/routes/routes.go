package routes

import (
	"github.com/gin-gonic/gin"

	"ims/internal/handlers"
	"ims/internal/middleware"
	"ims/internal/ratelimit"
)

type Handlers struct {
	Health    *handlers.HealthHandler
	Ingestion *handlers.IngestionHandler
	WorkItems *handlers.WorkItemHandler
	RCA       *handlers.RCAHandler
}

func Register(router *gin.Engine, handlers Handlers, limiter ratelimit.RateLimiter) {
	router.GET("/health", handlers.Health.GetHealth)
	router.POST("/ingest", middleware.RateLimit(limiter, "ingest"), handlers.Ingestion.Ingest)
	if handlers.WorkItems != nil {
		router.GET("/work-items", handlers.WorkItems.ListActive)
		router.GET("/work-items/:id", handlers.WorkItems.GetByID)
		router.PATCH("/work-items/:id/status", handlers.WorkItems.UpdateStatus)
	}
	if handlers.RCA != nil {
		router.POST("/rca", handlers.RCA.Create)
	}
}
