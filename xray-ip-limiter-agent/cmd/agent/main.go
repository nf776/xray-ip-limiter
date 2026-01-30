package main

import (
	"log/slog"
	"os"

	"xray-ip-limiter-agent/internal/app"
	"xray-ip-limiter-agent/internal/utils/logger"
)

var Version = "dev"

func main() {
	if err := app.Run(Version); err != nil {
		slog.Error("application failed", logger.Err(err))
		os.Exit(1)
	}
}
