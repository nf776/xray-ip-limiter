package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"xray-ip-limiter-agent/internal/agent/models"
	"xray-ip-limiter-agent/internal/agent/parser"
	"xray-ip-limiter-agent/internal/agent/sender"
	"xray-ip-limiter-agent/internal/config"
	"xray-ip-limiter-agent/internal/utils/logger"
)

type App struct {
	cfg    *config.Config
	parser *parser.LogParser
	sender *sender.NATSClient
}

func initFirewall() error {
	slog.Info("Initializing firewall rules...")

	delete := exec.Command("ipset", "destroy", "blacklist")
	output, err := delete.CombinedOutput()
	if strings.Contains(string(output), "it is in use by a kernel component") {
		slog.Info("Ipset already exists, skip")
	} else {
		if err != nil {
			slog.Error("failed to delete ipset", logger.Err(err), slog.String("output", string(output)))
			return err
		}
	}

	create := exec.Command("ipset", "create", "blacklist", "hash:ip", "timeout", "0")
	output, err = create.CombinedOutput()
	if !strings.Contains(string(output), "set with the same name already exists") {
		if err != nil {
			slog.Error("failed to create ipset", logger.Err(err), slog.String("output", string(output)))
			return err
		}
	}

	check := exec.Command("iptables", "-C", "INPUT", "-m", "set", "--match-set", "blacklist", "src", "-j", "DROP")
	if err := check.Run(); err != nil {
		add := exec.Command("iptables", "-I", "INPUT", "-m", "set", "--match-set", "blacklist", "src", "-j", "DROP")
		if err := add.Run(); err != nil {
			slog.Error("failed to add iptables rule", logger.Err(err))
			return err
		}
	}

	return nil
}

func Init() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	initFirewall()

	cfg := config.MustLoad()

	logger.SetupLogger()

	app, err := New(cfg)
	if err != nil {
		return err
	}

	return app.Run(ctx)
}

func New(cfg *config.Config) (*App, error) {
	natsPub, err := sender.NewNATSClient(cfg.NATS, cfg.NodeID)
	if err != nil {
		return nil, err
	}

	logParser := parser.NewLogParser(
		cfg.LogPath,
		cfg.NodeID,
	)

	return &App{
		cfg:    cfg,
		parser: logParser,
		sender: natsPub,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	slog.Info("Xray IP Limiter Agent started",
		"node_id", a.cfg.NodeID,
		"log_path", a.cfg.LogPath,
	)

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Go(func() {
		if err := a.sender.Start(ctx); err != nil {
			errChan <- fmt.Errorf("sender error: %w", err)
		}
	})

	wg.Go(func() {
		handler := func(entry models.LogEntry) {
			a.sender.Send(entry)
		}

		if err := a.parser.Start(ctx, handler); err != nil {
			errChan <- fmt.Errorf("parser error: %w", err)
		}
	})

	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully...")
	case err := <-errChan:
		slog.Error("application error", logger.Err(err))
		return err
	}

	a.parser.Stop()

	wg.Wait()

	return nil
}
