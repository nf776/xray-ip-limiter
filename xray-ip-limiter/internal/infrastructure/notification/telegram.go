package notification

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xray-ip-limiter/internal/usecase"
)

var _ usecase.Notifier = (*TelegramNotifier)(nil)

type TelegramConfig struct {
	BotToken string
	ChatID   string
	Enabled  bool
}

type TelegramNotifier struct {
	cfg    TelegramConfig
	client *http.Client
}

func NewTelegramNotifier(cfg TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramNotifier) NotifyStartup(ctx context.Context) error {
	if !t.cfg.Enabled {
		return nil
	}

	message := "✅ <b>IP Limiter Started</b>\n\nService is now running and monitoring connections."
	return t.sendMessage(ctx, message)
}

func (t *TelegramNotifier) NotifyLimitExceeded(
	ctx context.Context,
	userID string,
	ips []string,
	violationDuration string,
) error {
	if !t.cfg.Enabled {
		return nil
	}

	message := t.formatMessage(userID, ips, violationDuration)
	return t.sendMessage(ctx, message)
}

func (t *TelegramNotifier) formatMessage(userID string, ips []string, violationDuration string) string {
	var sb strings.Builder
	sb.WriteString("🚫 <b>IP Limit Exceeded</b>\n\n")
	sb.WriteString(fmt.Sprintf("<b>User ID:</b> <code>%s</code>\n", userID))
	sb.WriteString(fmt.Sprintf("<b>IP Count:</b> %d\n\n", len(ips)))
	sb.WriteString("<b>IP Addresses:</b>\n")
	for _, ip := range ips {
		sb.WriteString(fmt.Sprintf("• <code>%s</code>\n", ip))
	}
	sb.WriteString(fmt.Sprintf("Violation duration: %s", violationDuration))
	return sb.String()
}

func (t *TelegramNotifier) sendMessage(ctx context.Context, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)

	data := url.Values{}
	data.Set("chat_id", t.cfg.ChatID)
	data.Set("text", text)
	data.Set("parse_mode", "HTML")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api returned status: %d", resp.StatusCode)
	}

	return nil
}
