package domain

import "context"

type IPEventPublisher interface {
	Publish(ctx context.Context, entry LogEntry) error
}

type BlockCommandSubscriber interface {
	Subscribe(ctx context.Context, handler func(BlockCommand)) error
}

type IPBlocker interface {
	BlockIPs(ips []string, durationSec int) error
}

type LogParser interface {
	Start(ctx context.Context, handler func(LogEntry)) error
	Stop()
}
