package weather

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache stores cached NWS responses in Redis.
type RedisCache struct {
	RDB *redis.Client
}

func (r RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.RDB.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (r RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.RDB.Set(ctx, key, value, ttl).Err()
}
