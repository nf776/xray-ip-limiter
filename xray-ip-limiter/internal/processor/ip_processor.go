package processor

import (
	"context"
	"fmt"
	"log/slog"
	"xray-ip-limiter/internal/config"
	"xray-ip-limiter/internal/models"
	"xray-ip-limiter/internal/storage"
)

type IPProcessor struct {
	redis  *storage.RedisClient
	config config.ServiceConfig
}

func NewIPProcessor(redis *storage.RedisClient, cfg config.ServiceConfig) *IPProcessor {
	return &IPProcessor{
		redis:  redis,
		config: cfg,
	}
}

func (p *IPProcessor) ProcessUserIP(ctx context.Context, event models.UserIPEvent) (*models.BlockCommand, error) {
	count, err := p.redis.AddAndGetCount(ctx, event.UserID, event.IP, p.config.BanDuration*2)
	if err != nil {
		return nil, fmt.Errorf("failed to process redis: %w", err)
	}

	if int(count) <= p.config.IPLimit {
		return nil, nil
	}

	lockKey := fmt.Sprintf("lock:block:%s", event.UserID)
	isNewBlock, err := p.redis.SetLock(ctx, lockKey, p.config.BanDuration)
	if err != nil {
		return nil, err
	}

	if !isNewBlock {
		return nil, nil
	}

	ips, err := p.redis.GetUserIPs(ctx, event.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ips: %w", err)
	}

	slog.Warn("IP LIMIT EXCEEDED",
		"user_id", event.UserID,
		"unique_ips", count,
		"action", "sending_block_command",
	)

	return &models.BlockCommand{
		UserID:     event.UserID,
		BlockedIPs: ips,
		Action:     "block",
		Duration:   int(p.config.BanDuration.Seconds()),
		Timestamp:  event.Timestamp,
	}, nil
}
