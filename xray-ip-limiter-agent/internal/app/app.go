package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"xray-ip-limiter-agent/internal/infrastructure/config"
	"xray-ip-limiter-agent/internal/infrastructure/firewall"
	"xray-ip-limiter-agent/internal/infrastructure/logparser"
	"xray-ip-limiter-agent/internal/infrastructure/messaging"
	"xray-ip-limiter-agent/internal/usecase"
	"xray-ip-limiter-agent/internal/utils/logger"
)

type App struct {
	cfg        *config.Config
	natsClient *messaging.NATSClient
	agent      *usecase.Agent
	blocker    *firewall.IPSetBlocker
	parser     *logparser.TailParser
}

func Run(version string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := config.MustLoad()

	logger.SetupLogger()

	blocker := firewall.NewIPSetBlocker(cfg.Debug.DisableBlock)
	if err := blocker.Init(); err != nil {
		slog.Warn("firewall init failed, continuing anyway", slog.String("error", err.Error()))
	}

	app, err := New(cfg, blocker)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	defer app.Shutdown()

	return app.Start(ctx, version)
}

func New(cfg *config.Config, blocker *firewall.IPSetBlocker) (*App, error) {
	natsClient, err := messaging.NewNATSClient(messaging.NATSClientConfig{
		URL:    cfg.NATS.URL,
		Token:  cfg.NATS.Token,
		NodeID: cfg.NodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init nats: %w", err)
	}
	slog.Info("Connected to NATS")

	publisher := messaging.NewNATSPublisher(natsClient.JetStream())

	subscriber := messaging.NewNATSSubscriber(natsClient.JetStream(), cfg.NodeID)

	parser := logparser.NewTailParser(cfg.LogPath, cfg.NodeID)

	agent := usecase.NewAgent(
		parser,
		publisher,
		subscriber,
		blocker,
	)

	return &App{
		cfg:        cfg,
		natsClient: natsClient,
		agent:      agent,
		blocker:    blocker,
		parser:     parser,
	}, nil
}

func (a *App) Start(ctx context.Context, version string) error {
	slog.Info("Xray IP Limiter Agent started",
		slog.String("version", version),
		slog.String("node_id", a.cfg.NodeID),
		slog.String("log_path", a.cfg.LogPath),
	)

	return a.agent.Run(ctx)
}

func (a *App) Shutdown() {
	if a.natsClient != nil {
		a.natsClient.Close()
	}
	slog.Info("Agent stopped")
}
