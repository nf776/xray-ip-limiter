package persistence

import (
	"context"
	"fmt"
	"time"

	"xray-ip-limiter/internal/domain/repository"

	"github.com/redis/go-redis/v9"
)

var _ repository.SessionRepository = (*RedisRepository)(nil)
var _ repository.LockRepository = (*RedisRepository)(nil)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(cfg RedisConfig) (*RedisRepository, error) {
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

	return &RedisRepository{client: client}, nil
}

func (r *RedisRepository) Close() error {
	return r.client.Close()
}

func (r *RedisRepository) AddUserIP(ctx context.Context, userID, ip string, window time.Duration) (int, error) {
	key := r.buildUserIPsKey(userID)
	now := time.Now().Unix()
	min := now - int64(window.Seconds())

	pipe := r.client.TxPipeline()

	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: ip})
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", min))
	countRes := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	return int(countRes.Val()), nil
}

func (r *RedisRepository) GetUserIPs(ctx context.Context, userID string) ([]string, error) {
	key := r.buildUserIPsKey(userID)
	return r.client.ZRange(ctx, key, 0, -1).Result()
}

func (r *RedisRepository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}

func (r *RedisRepository) ReleaseLock(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisRepository) buildUserIPsKey(userID string) string {
	return fmt.Sprintf("user:%s:ips", userID)
}
