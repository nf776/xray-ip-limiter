package main

import (
	"log/slog"
	"os"
	"xray-ip-limiter-agent/internal/app"
	"xray-ip-limiter-agent/internal/utils/logger"
)

func main() {
	if err := app.Init(); err != nil {
		slog.Error("application failed", logger.Err(err))
		os.Exit(1)
	}
}
