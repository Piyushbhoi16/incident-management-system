package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"ims/internal/domain"
)

var ErrWorkItemCacheMiss = errors.New("work item cache miss")

type WorkItemCache interface {
	GetActive(ctx context.Context) ([]domain.WorkItem, error)
	SetActive(ctx context.Context, items []domain.WorkItem, ttl time.Duration) error
	DeleteActive(ctx context.Context) error
}

type RedisWorkItemCache struct {
	client *redis.Client
	key    string
}

type cachedWorkItem struct {
	ID          string    `json:"id"`
	ComponentID string    `json:"component_id"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewRedisWorkItemCache(client *redis.Client) *RedisWorkItemCache {
	return &RedisWorkItemCache{
		client: client,
		key:    "dashboard:active_work_items",
	}
}

func (c *RedisWorkItemCache) GetActive(ctx context.Context) ([]domain.WorkItem, error) {
	value, err := c.client.Get(ctx, c.key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrWorkItemCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("get active work items cache: %w", err)
	}

	var cached []cachedWorkItem
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return nil, fmt.Errorf("decode active work items cache: %w", err)
	}

	items := make([]domain.WorkItem, 0, len(cached))
	for _, item := range cached {
		items = append(items, domain.WorkItem{
			ID:          item.ID,
			ComponentID: item.ComponentID,
			Severity:    domain.Severity(item.Severity),
			Status:      domain.WorkItemStatus(item.Status),
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return items, nil
}

func (c *RedisWorkItemCache) SetActive(ctx context.Context, items []domain.WorkItem, ttl time.Duration) error {
	cached := make([]cachedWorkItem, 0, len(items))
	for _, item := range items {
		cached = append(cached, cachedWorkItem{
			ID:          item.ID,
			ComponentID: item.ComponentID,
			Severity:    string(item.Severity),
			Status:      string(item.Status),
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	payload, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("encode active work items cache: %w", err)
	}
	if err := c.client.Set(ctx, c.key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("set active work items cache: %w", err)
	}

	return nil
}

func (c *RedisWorkItemCache) DeleteActive(ctx context.Context) error {
	if err := c.client.Del(ctx, c.key).Err(); err != nil {
		return fmt.Errorf("delete active work items cache: %w", err)
	}

	return nil
}
