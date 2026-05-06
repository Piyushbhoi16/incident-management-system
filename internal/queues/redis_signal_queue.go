package queues

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"ims/internal/domain"
)

type RedisSignalQueue struct {
	client  *redis.Client
	name    string
	dlqName string
}

func NewRedisSignalQueue(client *redis.Client, name string, dlqName ...string) *RedisSignalQueue {
	resolvedDLQName := name + ":dlq"
	if len(dlqName) > 0 && dlqName[0] != "" {
		resolvedDLQName = dlqName[0]
	}

	return &RedisSignalQueue{
		client:  client,
		name:    name,
		dlqName: resolvedDLQName,
	}
}

func (q *RedisSignalQueue) Enqueue(ctx context.Context, signal domain.Signal) error {
	payload, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}

	if err := q.client.RPush(ctx, q.name, payload).Err(); err != nil {
		return fmt.Errorf("push signal to redis queue: %w", err)
	}

	return nil
}

func (q *RedisSignalQueue) Dequeue(ctx context.Context) ([]byte, error) {
	result, err := q.client.BLPop(ctx, 0, q.name).Result()
	if err != nil {
		return nil, fmt.Errorf("pop signal from redis queue: %w", err)
	}

	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected redis queue response length: %d", len(result))
	}

	return []byte(result[1]), nil
}

func (q *RedisSignalQueue) EnqueueRaw(ctx context.Context, payload []byte) error {
	if err := q.client.RPush(ctx, q.dlqName, payload).Err(); err != nil {
		return fmt.Errorf("push signal to redis dlq: %w", err)
	}

	return nil
}
