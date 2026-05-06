package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRateLimiter struct {
	client    *redis.Client
	limit     int
	window    time.Duration
	keyPrefix string
}

func NewRedisRateLimiter(client *redis.Client, limit int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:    client,
		limit:     limit,
		window:    window,
		keyPrefix: "rate_limit",
	}
}

func (l *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)

	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("increment rate limit counter: %w", err)
	}

	if count == 1 {
		if err := l.client.Expire(ctx, redisKey, l.window).Err(); err != nil {
			return false, fmt.Errorf("set rate limit expiry: %w", err)
		}
	}

	return count <= int64(l.limit), nil
}
