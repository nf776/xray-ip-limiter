package usecase

import "context"

type BlockPublisher interface {
	PublishBlock(ctx context.Context, cmd BlockCommandOutput) error
}

type Notifier interface {
	NotifyLimitExceeded(ctx context.Context, userID string, ips []string, limit int, violationDuration string) error
	NotifyStartup(ctx context.Context) error
}

type RemnawaveClient interface {
	FetchUsersLimits(ctx context.Context) (map[int]int, error)
}
