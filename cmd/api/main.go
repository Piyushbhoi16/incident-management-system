package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"ims/internal/config"
	"ims/internal/queues"
	"ims/internal/repositories"
	"ims/internal/server"
	"ims/internal/services"
	"ims/internal/worker"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}

	// Single pool shared by HTTP handlers (future) and the signal worker for WorkItem writes.
	postgresPool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}

	signalQueue := queues.NewRedisSignalQueue(redisClient, cfg.SignalQueueName, cfg.SignalDLQName)
	rawSignals := repositories.NewMongoRawSignalRepository(
		mongoClient.Database(cfg.MongoDatabase).Collection(cfg.MongoCollection),
	)
	workItemsRepo := repositories.NewPostgresWorkItemRepository(postgresPool)
	rcaRepo := repositories.NewPostgresRCARepository(postgresPool)
	workItemsCache := repositories.NewRedisWorkItemCache(redisClient)
	workItemsService := services.NewWorkItemServiceWithCache(workItemsRepo, rcaRepo, workItemsCache)
	rcaService := services.NewRCAService(rcaRepo, workItemsRepo)

	indexCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if err := rawSignals.EnsureIndexes(indexCtx); err != nil {
		cancel()
		log.Fatalf("ensure raw signal indexes: %v", err)
	}
	if err := workItemsRepo.EnsureSchema(indexCtx); err != nil {
		cancel()
		log.Fatalf("ensure work item schema: %v", err)
	}
	if err := rcaRepo.EnsureSchema(indexCtx); err != nil {
		cancel()
		log.Fatalf("ensure rca schema: %v", err)
	}
	cancel()

	debounceRepo := repositories.NewRedisDebounceRepository(redisClient)
	debounceService := services.NewDebounceService(debounceRepo)
	signalWorker := worker.NewSignalWorker(signalQueue, rawSignals, debounceService, workItemsService, cfg.WorkerCount)
	signalWorker.Start(ctx)

	app := server.NewWithWorkItems(cfg, workItemsService, rcaService)

	log.Printf("starting ims api on %s", cfg.HTTPAddr)
	if err := app.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("api server stopped: %v", err)
	}
}
