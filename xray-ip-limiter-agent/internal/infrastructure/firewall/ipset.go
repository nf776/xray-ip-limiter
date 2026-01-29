package firewall

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"xray-ip-limiter-agent/internal/domain"
)

var _ domain.IPBlocker = (*IPSetBlocker)(nil)

const (
	defaultTimeout = 600
	blacklistName  = "blacklist"
)

type IPSetBlocker struct {
	IsBlockDisabled bool
}

func NewIPSetBlocker(disableBlock bool) *IPSetBlocker {
	return &IPSetBlocker{
		IsBlockDisabled: disableBlock,
	}
}

func (b *IPSetBlocker) Init() error {
	slog.Info("Initializing firewall rules...")

	destroy := exec.Command("ipset", "destroy", blacklistName)
	output, err := destroy.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if !strings.Contains(outStr, "in use") && !strings.Contains(outStr, "does not exist") {
			slog.Debug("ipset destroy", slog.String("output", outStr))
		}
	}

	create := exec.Command("ipset", "create", blacklistName, "hash:ip", "timeout", "0")
	output, err = create.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if !strings.Contains(outStr, "already exists") {
			slog.Error("failed to create ipset", slog.String("output", outStr))
			return fmt.Errorf("failed to create ipset: %w", err)
		}
	}

	check := exec.Command("iptables", "-C", "INPUT", "-m", "set", "--match-set", blacklistName, "src", "-j", "DROP")
	if err := check.Run(); err != nil {
		add := exec.Command("iptables", "-I", "INPUT", "-m", "set", "--match-set", blacklistName, "src", "-j", "DROP")
		if err := add.Run(); err != nil {
			slog.Error("failed to add iptables rule")
			return fmt.Errorf("failed to add iptables rule: %w", err)
		}
	}

	slog.Info("Firewall initialized successfully")
	return nil
}

func (b *IPSetBlocker) BlockIPs(ips []string, durationSec int) error {
	if b.IsBlockDisabled {
		return nil
	}

	timeout := defaultTimeout
	if durationSec > 0 {
		timeout = durationSec
	}

	timeoutStr := fmt.Sprintf("%d", timeout)

	for _, ip := range ips {
		cmd := exec.Command("ipset", "add", blacklistName, ip, "timeout", timeoutStr, "-exist")
		if err := cmd.Run(); err != nil {
			slog.Error("ipset add failed", slog.String("ip", ip), slog.String("error", err.Error()))
			continue
		}
		slog.Info("IP blocked", slog.String("ip", ip), slog.Int("timeout_sec", timeout))
	}

	return nil
}
