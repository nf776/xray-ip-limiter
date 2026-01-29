package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"xray-ip-limiter/internal/infrastructure/config"
	"xray-ip-limiter/internal/infrastructure/messaging"
	"xray-ip-limiter/internal/infrastructure/notification"
	"xray-ip-limiter/internal/infrastructure/persistence"
	"xray-ip-limiter/internal/usecase"
	"xray-ip-limiter/internal/utils/logger"
)

type App struct {
	cfg        *config.Config
	redis      *persistence.RedisRepository
	natsClient *messaging.NATSClient
	subscriber *messaging.NATSSubscriber
	notifier   *notification.TelegramNotifier
}

func Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := config.MustLoad()

	logger.SetupLogger()

	app, err := New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	defer app.Shutdown()

	return app.Start(ctx)
}

func New(cfg *config.Config) (*App, error) {
	redisRepo, err := persistence.NewRedisRepository(persistence.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init redis: %w", err)
	}
	slog.Info("Connected to Redis")

	natsClient, err := messaging.NewNATSClient(messaging.NATSClientConfig{
		URL:   cfg.NATS.URL,
		Token: cfg.NATS.Token,
		Name:  "xray-ip-limiter",
	})
	if err != nil {
		redisRepo.Close()
		return nil, fmt.Errorf("failed to init nats: %w", err)
	}
	slog.Info("Connected to NATS")

	publisher := messaging.NewNATSPublisher(natsClient.JetStream(), "xray-observer-block")

	notifier := notification.NewTelegramNotifier(notification.TelegramConfig{
		Enabled:  cfg.Telegram.Enabled,
		BotToken: cfg.Telegram.BotToken,
		ChatID:   cfg.Telegram.ChatID,
	})

	ipLimiter := usecase.NewIPLimiter(
		redisRepo,
		redisRepo,
		publisher,
		notifier,
		usecase.IPLimiterConfig{
			IPLimit:     cfg.Service.IPLimit,
			BanDuration: cfg.Service.BanDuration,
		},
	)

	subscriber := messaging.NewNATSSubscriber(
		natsClient.JetStream(),
		messaging.NATSSubscriberConfig{
			WorkersCount: cfg.NATS.WorkersCount,
			StreamName:   "OBSERVER_STREAM",
			Subject:      "xray-observer-ip",
			ConsumerName: "observer-cons-1",
		},
		ipLimiter,
	)

	return &App{
		cfg:        cfg,
		redis:      redisRepo,
		natsClient: natsClient,
		subscriber: subscriber,
		notifier:   notifier,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	slog.Info(
		"IP Limiter started",
		slog.Int("ip_limit", a.cfg.Service.IPLimit),
		slog.Float64("ban_duration_sec", a.cfg.Service.BanDuration.Seconds()),
	)

	if err := a.notifier.NotifyStartup(ctx); err != nil {
		slog.Error("failed to send startup notification", slog.String("error", err.Error()))
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	wg.Go(func() {
		if err := a.subscriber.Start(ctx); err != nil {
			errChan <- err
		}
	})

	select {
	case <-ctx.Done():
		slog.Info("Shutting down gracefully...")
	case err := <-errChan:
		return err
	}

	wg.Wait()
	return nil
}

func (a *App) Shutdown() {
	if a.natsClient != nil {
		a.natsClient.Close()
	}

	if a.redis != nil {
		if err := a.redis.Close(); err != nil {
			slog.Error("failed to close redis connection", slog.String("error", err.Error()))
		}
	}

	slog.Info("Application stopped")
}
