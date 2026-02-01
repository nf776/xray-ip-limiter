package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"xray-ip-limiter/internal/infrastructure/clients"
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
	syncer     *usecase.LimitSyncer
}

func Run(version string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := config.MustLoad()

	logger.SetupLogger()

	app, err := New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	defer app.Shutdown()

	return app.Start(ctx, version)
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

	var remnawaveSyncer bool = cfg.Remnawave.Enabled
	var memoryLimiterRepo *persistence.MemoryLimitRepository
	var syncer *usecase.LimitSyncer

	if remnawaveSyncer {
		memoryLimiterRepo = persistence.NewMemoryLimitRepository()

		remnaClient := clients.NewRemnawaveClient(clients.RemnawaveConfig{
			URL:          cfg.Remnawave.URL,
			ApiKey:       cfg.Remnawave.ApiKey,
			DefaultLimit: cfg.Service.IPLimit,
			EGamesCookies: clients.CookiesConfig{
				Enabled: cfg.Remnawave.EGamesCookies.Enabled,
				Name:    cfg.Remnawave.EGamesCookies.Name,
				Value:   cfg.Remnawave.EGamesCookies.Value,
			},
		})

		syncer = usecase.NewLimitSyncer(remnaClient, memoryLimiterRepo)
	}

	publisher := messaging.NewNATSPublisher(natsClient.JetStream(), "xray-observer-block")

	notifier := notification.NewTelegramNotifier(notification.TelegramConfig{
		Enabled:  cfg.Telegram.Enabled,
		BotToken: cfg.Telegram.BotToken,
		ChatID:   cfg.Telegram.ChatID,
	})

	ipLimiter := usecase.NewIPLimiter(
		memoryLimiterRepo,
		redisRepo,
		redisRepo,
		redisRepo,
		publisher,
		notifier,
		usecase.IPLimiterConfig{
			IPLimit:         cfg.Service.IPLimit,
			BanDuration:     cfg.Service.BanDuration,
			RemnawaveSyncer: remnawaveSyncer,
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
		syncer:     syncer,
	}, nil
}

func (a *App) Start(ctx context.Context, version string) error {
	slog.Info(
		"IP Limiter started",
		slog.String("version", version),
		slog.Int("ip_limit", a.cfg.Service.IPLimit),
		slog.Float64("ban_duration_sec", a.cfg.Service.BanDuration.Seconds()),
	)

	if err := a.notifier.NotifyStartup(ctx); err != nil {
		slog.Error("failed to send startup notification", slog.String("error", err.Error()))
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	if a.syncer != nil {
		wg.Go(func() {
			a.syncer.Run(ctx)
		})
	}

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
