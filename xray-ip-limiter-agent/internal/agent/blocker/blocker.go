package blocker

import (
	"fmt"
	"log/slog"
	"os/exec"
	"xray-ip-limiter-agent/internal/agent/models"
	"xray-ip-limiter-agent/internal/utils/logger"
)

func BlockIPs(cmd models.BlockCommand) {
	timeout := 600
	if cmd.Duration > 0 {
		timeout = cmd.Duration
	}

	timeoutStr := fmt.Sprintf("%d", timeout)

	for _, ip := range cmd.BlockedIPs {
		c := exec.Command("ipset", "add", "blacklist", ip, "timeout", timeoutStr, "-exist")

		if err := c.Run(); err != nil {
			slog.Error("ipset failed", slog.String("ip", ip), logger.Err(err))
			continue
		}
		slog.Info("IPs blocked",
			slog.String("ip", ip),
			slog.Int("timeout_sec", timeout),
			slog.String("user", cmd.UserID),
		)
	}
}
