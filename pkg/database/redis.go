// Package database provides shared database utilities for all services.
package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// NewRedisClient creates a new Redis client.
func NewRedisClient(ctx context.Context, cfg RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}

// NewRedisClientFromURL creates a Redis client from a connection URL.
func NewRedisClientFromURL(ctx context.Context, redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}
