package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DebounceRepository coordinates per-component burst debouncing. The work_item_id
// stored in Redis under debounce:{component_id} is the single source of truth for
// that debounce window; callers must use the returned id for downstream storage.
type DebounceRepository interface {
	GetOrCreateWorkItemID(ctx context.Context, componentID string, newWorkItemID string, ttl time.Duration) (workItemID string, created bool, err error)
}

type RedisDebounceRepository struct {
	client *redis.Client
}

func NewRedisDebounceRepository(client *redis.Client) *RedisDebounceRepository {
	return &RedisDebounceRepository{client: client}
}

// debounceGetOrSetScript atomically sets debounce:{component} to newWorkItemID with TTL
// if the key is absent; otherwise returns the existing value. The returned id is always
// the value held in Redis after the script runs.
const debounceGetOrSetScript = `
local ok = redis.call('SET', KEYS[1], ARGV[1], 'NX', 'PX', ARGV[2])
if ok then
  return {ARGV[1], "1"}
end
local existing = redis.call('GET', KEYS[1])
if not existing then
  return redis.error_reply("debounce: key missing after failed SET NX")
end
return {existing, "0"}
`

func (r *RedisDebounceRepository) GetOrCreateWorkItemID(ctx context.Context, componentID string, newWorkItemID string, ttl time.Duration) (string, bool, error) {
	key := fmt.Sprintf("debounce:%s", componentID)
	ttlMs := ttl.Milliseconds()
	if ttlMs <= 0 {
		return "", false, fmt.Errorf("debounce ttl must be positive")
	}

	raw, err := r.client.Eval(ctx, debounceGetOrSetScript, []string{key}, newWorkItemID, ttlMs).Result()
	if err != nil {
		return "", false, fmt.Errorf("debounce get or create: %w", err)
	}

	parts, ok := raw.([]interface{})
	if !ok || len(parts) != 2 {
		return "", false, fmt.Errorf("debounce get or create: unexpected result %T", raw)
	}

	workItemID, err := redisBulkString(parts[0])
	if err != nil {
		return "", false, fmt.Errorf("debounce get or create: work_item_id: %w", err)
	}

	flag, err := redisBulkString(parts[1])
	if err != nil {
		return "", false, fmt.Errorf("debounce get or create: flag: %w", err)
	}

	created := flag == "1"
	return workItemID, created, nil
}

func redisBulkString(v interface{}) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	default:
		return "", fmt.Errorf("unexpected type %T", x)
	}
}
