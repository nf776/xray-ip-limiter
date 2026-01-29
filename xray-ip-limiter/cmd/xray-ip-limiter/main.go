package main

import (
	"log/slog"
	"os"

	"xray-ip-limiter/internal/app"
	"xray-ip-limiter/internal/utils/logger"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("application failed", logger.Err(err))
		os.Exit(1)
	}
}
