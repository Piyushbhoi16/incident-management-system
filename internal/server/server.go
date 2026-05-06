package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"ims/internal/config"
	"ims/internal/handlers"
	"ims/internal/middleware"
	"ims/internal/queues"
	"ims/internal/ratelimit"
	"ims/internal/repositories"
	"ims/internal/routes"
	"ims/internal/services"
)

func New(cfg config.Config) *gin.Engine {
	return newRouter(cfg, nil, nil)
}

func NewWithWorkItems(cfg config.Config, workItemsService *services.WorkItemService, rcaService *services.RCAService) *gin.Engine {
	return newRouter(cfg, workItemsService, rcaService)
}

func newRouter(cfg config.Config, workItemsService *services.WorkItemService, rcaService *services.RCAService) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(middleware.CORS(), middleware.Logging(), gin.Recovery())

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	healthRepo := repositories.NewInMemoryHealthRepository(cfg)
	healthService := services.NewHealthService(healthRepo)
	healthHandler := handlers.NewHealthHandler(healthService)

	signalQueue := queues.NewRedisSignalQueue(redisClient, cfg.SignalQueueName, cfg.SignalDLQName)
	ingestionService := services.NewIngestionService(signalQueue)
	ingestionHandler := handlers.NewIngestionHandler(ingestionService)

	var workItemHandler *handlers.WorkItemHandler
	if workItemsService != nil {
		workItemHandler = handlers.NewWorkItemHandler(workItemsService)
	}
	var rcaHandler *handlers.RCAHandler
	if rcaService != nil {
		rcaHandler = handlers.NewRCAHandler(rcaService)
	}

	rateLimitWindow, err := time.ParseDuration(cfg.RateLimitWindow)
	if err != nil {
		rateLimitWindow = time.Second
	}
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient, cfg.RateLimitRequests, rateLimitWindow)

	routes.Register(router, routes.Handlers{
		Health:    healthHandler,
		Ingestion: ingestionHandler,
		WorkItems: workItemHandler,
		RCA:       rcaHandler,
	}, rateLimiter)

	return router
}
