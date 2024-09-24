package cache

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisCache(addr, password string, db int) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	return &RedisCache{client: rdb, ctx: ctx}
}

func (r *RedisCache) Get(key string) (int64, error) {
	return r.client.Get(r.ctx, key).Int64()
}

func (r *RedisCache) Set(key string, value int64, expiration time.Duration) error {
	return r.client.Set(r.ctx, key, value, expiration).Err()
}

func (r *RedisCache) IncrBy(key string, value int64) error {
	return r.client.IncrBy(r.ctx, key, value).Err()
}
