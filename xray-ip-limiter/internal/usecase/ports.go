package usecase

import "context"

type Notifier interface {
	NotifyLimitExceeded(ctx context.Context, userID string, ips []string) error
	NotifyStartup(ctx context.Context) error
}
