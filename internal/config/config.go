package config

import (
	"os"
	"strconv"

	"ims/internal/constants"
)

type Config struct {
	AppName           string
	Env               string
	HTTPAddr          string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	MongoURI          string
	MongoDatabase     string
	MongoCollection   string
	// PostgresDSN is a libpq-style connection string (e.g. postgres://user:pass@host:5432/db?sslmode=disable).
	// Used by the API process for work_items and by the embedded signal worker in cmd/api.
	PostgresDSN       string
	SignalQueueName   string
	SignalDLQName     string
	WorkerCount       int
	RateLimitRequests int
	RateLimitWindow   string
}

func Load() Config {
	return Config{
		AppName:           getEnv("APP_NAME", "ims-backend"),
		Env:               getEnv("APP_ENV", "development"),
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvInt("REDIS_DB", 0),
		MongoURI:          getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:     getEnv("MONGO_DATABASE", "ims"),
		MongoCollection:   getEnv("MONGO_COLLECTION", constants.RawSignalsCollection),
		PostgresDSN:       getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/ims?sslmode=disable"),
		SignalQueueName:   getEnv("SIGNAL_QUEUE_NAME", constants.SignalQueueName),
		SignalDLQName:     getEnv("SIGNAL_DLQ_NAME", constants.SignalDLQName),
		WorkerCount:       getEnvInt("WORKER_COUNT", 5),
		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getEnv("RATE_LIMIT_WINDOW", "1s"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
