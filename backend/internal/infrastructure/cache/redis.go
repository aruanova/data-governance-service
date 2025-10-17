package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alejandroruanova/data-governance-service/backend/internal/pkg/config"
	"github.com/redis/go-redis/v9"
)

// RedisCache wraps the Redis client
type RedisCache struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(cfg *config.CacheConfig, logger *slog.Logger) (*RedisCache, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	logger.Info("redis connection established",
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.Int("db", cfg.DB),
	)

	return &RedisCache{
		client: client,
		logger: logger,
	}, nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	r.logger.Info("closing redis connection")
	return r.client.Close()
}

// Set stores a value in cache with TTL
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value from cache
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// GetBytes retrieves bytes from cache
func (r *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return r.client.Get(ctx, key).Bytes()
}

// Delete removes a key from cache
func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (r *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// Expire sets a timeout on a key
func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// HSet sets a hash field
func (r *RedisCache) HSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.HSet(ctx, key, values...).Err()
}

// HGet gets a hash field
func (r *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

// HGetAll gets all hash fields
func (r *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel deletes hash fields
func (r *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// Incr increments a counter
func (r *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Decr decrements a counter
func (r *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// SAdd adds members to a set
func (r *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SMembers gets all set members
func (r *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SIsMember checks if a member exists in a set
func (r *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// ZAdd adds members to a sorted set
func (r *RedisCache) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

// ZRange gets members from sorted set by range
func (r *RedisCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

// Ping checks if Redis is alive
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Health returns health status of Redis
func (r *RedisCache) Health(ctx context.Context) map[string]interface{} {
	stats := r.client.PoolStats()

	return map[string]interface{}{
		"status":       "up",
		"hits":         stats.Hits,
		"misses":       stats.Misses,
		"timeouts":     stats.Timeouts,
		"total_conns":  stats.TotalConns,
		"idle_conns":   stats.IdleConns,
		"stale_conns":  stats.StaleConns,
	}
}

// TTL returns the remaining time to live of a key
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// SetNX sets a key only if it doesn't exist (for distributed locks)
func (r *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

// GetSet atomically sets key to value and returns the old value
func (r *RedisCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	return r.client.GetSet(ctx, key, value).Result()
}

// Keys returns all keys matching pattern (use with caution in production)
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// FlushDB clears the current database (use with EXTREME caution)
func (r *RedisCache) FlushDB(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}