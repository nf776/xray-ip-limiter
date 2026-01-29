package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"xray-ip-limiter/internal/config"
	"xray-ip-limiter/internal/nats"
	"xray-ip-limiter/internal/processor"
	"xray-ip-limiter/internal/storage"
	"xray-ip-limiter/internal/infrastructure/notification"
	"xray-ip-limiter/internal/utils/logger"
)

type App struct {
	cfg       *config.Config
	redis     *storage.RedisClient
	processor *processor.IPProcessor
	reader    *nats.NATSClient
	notifier   *notification.TelegramNotifier
}

func Init() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := config.MustLoad()
	logger.SetupLogger()

	app, err := New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	return app.Run(ctx)
}

func New(cfg *config.Config) (*App, error) {
	redisClient, err := storage.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis: %w", err)
	}

	slog.Info("Connected to Redis")

	proc := processor.NewIPProcessor(redisClient, cfg.Service)

	notifier := notification.NewTelegramNotifier(notification.TelegramConfig{
		Enabled:  cfg.Telegram.Enabled,
		BotToken: cfg.Telegram.BotToken,
		ChatID:   cfg.Telegram.ChatID,
	})

	return &App{
		cfg:       cfg,
		redis:     redisClient,
		processor: proc,
		reader:    natsReader,
		notifier:   notifier,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	slog.Info(
		"Observer started",
		slog.Int("ip_limit", a.cfg.Service.IPLimit),
		slog.Float64("ban_duration", a.cfg.Service.BanDuration.Seconds()),
	)

	if err := a.notifier.NotifyStartup(ctx); err != nil {
		slog.Error("failed to send startup notification", slog.String("error", err.Error()))
	}
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Go(func() {
		if err := a.reader.Start(ctx); err != nil {
			errChan <- err
		}
	})

	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully...")
	case err := <-errChan:
		return err
	}

	if err := a.redis.Close(); err != nil {
		slog.Error("failed to close redis conn", logger.Err(err))
		return err
	}

	wg.Wait()

	return nil
}
