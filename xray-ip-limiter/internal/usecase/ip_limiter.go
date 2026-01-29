package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"xray-ip-limiter/internal/domain/entity"
	"xray-ip-limiter/internal/domain/repository"
)

type IPLimiterConfig struct {
	IPLimit     int
	BanDuration time.Duration
}

type IPLimiter struct {
	sessionRepo repository.SessionRepository
	lockRepo    repository.LockRepository
	publisher   BlockPublisher
	notifier    Notifier
	config      IPLimiterConfig
}

func NewIPLimiter(
	sessionRepo repository.SessionRepository,
	lockRepo repository.LockRepository,
	publisher BlockPublisher,
	notifier Notifier,
	config IPLimiterConfig,
) *IPLimiter {
	return &IPLimiter{
		sessionRepo: sessionRepo,
		lockRepo:    lockRepo,
		publisher:   publisher,
		notifier:    notifier,
		config:      config,
	}
}

func (uc *IPLimiter) ProcessIPEvent(ctx context.Context, input IPEventInput) (*BlockCommandOutput, error) {
	uniqueIPCount, err := uc.sessionRepo.AddUserIP(
		ctx,
		input.UserID,
		input.IP,
		uc.config.BanDuration*2,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add user IP: %w", err)
	}

	ips, err := uc.sessionRepo.GetUserIPs(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user IPs: %w", err)
	}

	slog.Info("IP event processed",
		slog.String("user_id", input.UserID),
		slog.String("new_ip", input.IP),
		slog.Int("unique_count", uniqueIPCount),
		slog.Int("limit", uc.config.IPLimit),
		slog.Any("all_ips", ips),
	)

	if uniqueIPCount <= uc.config.IPLimit {
		return nil, nil
	}

	lockKey := uc.buildLockKey(input.UserID, uniqueIPCount)
	acquired, err := uc.lockRepo.AcquireLock(ctx, lockKey, uc.config.BanDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return nil, nil
	}

	blockCmd, err := entity.NewBlockCommand(input.UserID, ips, uc.config.BanDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create block command: %w", err)
	}

	slog.Warn("IP limit exceeded",
		slog.String("user_id", input.UserID),
		slog.Int("unique_ips", uniqueIPCount),
		slog.Int("limit", uc.config.IPLimit),
	)

	if err := uc.notifier.NotifyLimitExceeded(ctx, input.UserID, ips); err != nil {
		slog.Error("failed to send notification", slog.String("error", err.Error()))
	}

	output := uc.toBlockCommandOutput(blockCmd, input.Timestamp)

	if err := uc.publisher.PublishBlock(ctx, output); err != nil {
		return nil, fmt.Errorf("failed to publish block command: %w", err)
	}

	slog.Info("Block command published",
		slog.String("user_id", input.UserID),
		slog.Int("blocked_ips", len(ips)),
	)

	return &output, nil
}

func (uc *IPLimiter) buildLockKey(userID string, ipCount int) string {
	return fmt.Sprintf("lock:block:%s:%d", userID, ipCount)
}

func (uc *IPLimiter) toBlockCommandOutput(cmd *entity.BlockCommand, timestamp time.Time) BlockCommandOutput {
	return BlockCommandOutput{
		UserID:     cmd.UserID(),
		BlockedIPs: cmd.BlockedIPs(),
		Action:     "block",
		Duration:   cmd.DurationSeconds(),
		Timestamp:  timestamp,
	}
}
