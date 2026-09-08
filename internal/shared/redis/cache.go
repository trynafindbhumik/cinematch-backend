package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrCacheDisabled = errors.New("redis cache disabled or unavailable")

// Get retrieves a JSON-marshaled item from Redis and unmarshals it into type T
func Get[T any](ctx context.Context, key string) (*T, error) {
	if client == nil {
		return nil, ErrCacheDisabled
	}

	data, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return nil, err
	}

	return &val, nil
}

// Set marshals value into JSON and stores it in Redis with the given TTL
func Set[T any](ctx context.Context, key string, value T, ttl time.Duration) error {
	if client == nil {
		return nil // Fail silently if Redis is disabled
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a key from Redis
func Delete(ctx context.Context, key string) error {
	if client == nil {
		return nil
	}
	return client.Del(ctx, key).Err()
}
