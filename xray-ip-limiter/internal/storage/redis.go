package storage

import (
	"context"
	"fmt"
	"time"
	"xray-ip-limiter/internal/config"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) AddUserIP(ctx context.Context, userID, ip string, ttl time.Duration) error {
	key := fmt.Sprintf("user:%s:ips", userID)
	score := float64(time.Now().Unix())

	if err := r.client.ZAddNX(ctx, key, redis.Z{
		Score:  score,
		Member: ip,
	}).Err(); err != nil {
		return err
	}

	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *RedisClient) GetUserIPCount(ctx context.Context, userID string, window time.Duration) (int, error) {
	key := fmt.Sprintf("user:%s:ips", userID)

	minScore := float64(time.Now().Add(-window).Unix())
	r.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%f", minScore))

	count, err := r.client.ZCard(ctx, key).Result()
	return int(count), err
}

func (r *RedisClient) GetUserIPs(ctx context.Context, userID string) ([]string, error) {
	key := fmt.Sprintf("user:%s:ips", userID)

	return r.client.ZRange(ctx, key, 0, -1).Result()
}

func (r *RedisClient) AddAndGetCount(ctx context.Context, userID, ip string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("user:%s:ips", userID)
	now := time.Now().Unix()
	min := now - int64(window.Seconds())

	pipe := r.client.TxPipeline()

	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: ip})

	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", min))

	countRes := pipe.ZCard(ctx, key)

	pipe.Expire(ctx, key, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return countRes.Val(), nil
}

func (r *RedisClient) SetLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}
