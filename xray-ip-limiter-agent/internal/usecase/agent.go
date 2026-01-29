package usecase

import (
	"context"
	"log/slog"
	"sync"

	"xray-ip-limiter-agent/internal/domain"
)

type Agent struct {
	parser     domain.LogParser
	publisher  domain.IPEventPublisher
	subscriber domain.BlockCommandSubscriber
	blocker    domain.IPBlocker
}

func NewAgent(
	parser domain.LogParser,
	publisher domain.IPEventPublisher,
	subscriber domain.BlockCommandSubscriber,
	blocker domain.IPBlocker,
) *Agent {
	return &Agent{
		parser:     parser,
		publisher:  publisher,
		subscriber: subscriber,
		blocker:    blocker,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Go(func() {
		err := a.parser.Start(ctx, func(entry domain.LogEntry) {
			if err := a.publisher.Publish(ctx, entry); err != nil {
				slog.Error("failed to publish IP event",
					slog.String("error", err.Error()),
					slog.String("user", entry.Email),
				)
			}
		})
		if err != nil {
			errChan <- err
		}
	})

	wg.Go(func() {
		err := a.subscriber.Subscribe(ctx, func(cmd domain.BlockCommand) {
			if err := a.blocker.BlockIPs(cmd.BlockedIPs, cmd.Duration); err != nil {
				slog.Error("failed to block IPs",
					slog.String("error", err.Error()),
					slog.String("user_id", cmd.UserID),
				)
			}
		})
		if err != nil {
			errChan <- err
		}
	})

	select {
	case <-ctx.Done():
		slog.Info("shutting down agent...")
	case err := <-errChan:
		return err
	}

	a.parser.Stop()
	wg.Wait()

	return nil
}
