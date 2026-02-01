package usecase

import (
	"context"
	"log/slog"
	"time"
	"xray-ip-limiter/internal/domain/repository"
	"xray-ip-limiter/internal/utils/logger"
)

type LimitSyncer struct {
	repo   repository.LimitsRepository
	client RemnawaveClient
}

func NewLimitSyncer(client RemnawaveClient, repo repository.LimitsRepository) *LimitSyncer {
	return &LimitSyncer{
		client: client,
		repo:   repo,
	}
}

func (s *LimitSyncer) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.sync(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.sync(ctx)
		}
	}
}

func (s *LimitSyncer) sync(ctx context.Context) {
	limits, err := s.client.FetchUsersLimits(ctx)
	if err != nil {
		slog.Error("failed to fetch limits from remnawave", logger.Err(err))
		return
	}

	s.repo.UpdateLimits(limits)

	slog.Debug("Limits syncronized successfully")
}
