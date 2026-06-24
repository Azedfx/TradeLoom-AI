package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisStateKey = "tradeloom_state"

type RedisStore struct {
	cli *redis.Client
}

func NewRedisStore(url string) (*RedisStore, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	opts.Protocol = 2
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 10 * time.Second
	opts.WriteTimeout = 10 * time.Second
	opts.PoolSize = 2
	cli := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{cli: cli}, nil
}

func (r *RedisStore) Load() (*storeSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	val, err := r.cli.Get(ctx, redisStateKey).Result()
	if err != nil {
		return nil, err
	}
	var snap storeSnapshot
	if err := json.Unmarshal([]byte(val), &snap); err != nil {
		return nil, fmt.Errorf("redis unmarshal: %w", err)
	}
	return &snap, nil
}

func (r *RedisStore) Save(snap *storeSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return r.SaveRaw(data)
}

func (r *RedisStore) SaveRaw(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.cli.Set(ctx, redisStateKey, string(data), 0).Err()
}

func (r *RedisStore) Close() error {
	return r.cli.Close()
}
