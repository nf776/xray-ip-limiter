package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"xray-ip-limiter-agent/internal/domain"

	"github.com/nats-io/nats.go"
)

var _ domain.IPEventPublisher = (*NATSPublisher)(nil)

const ipEventSubject = "xray-observer-ip"

type NATSPublisher struct {
	js nats.JetStreamContext
}

func NewNATSPublisher(js nats.JetStreamContext) *NATSPublisher {
	return &NATSPublisher{
		js: js,
	}
}

func (p *NATSPublisher) Publish(ctx context.Context, entry domain.LogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	_, err = p.js.Publish(ipEventSubject, data)
	if err != nil {
		slog.Error("failed to publish IP event", slog.String("error", err.Error()))
		return fmt.Errorf("failed to publish IP event: %w", err)
	}

	return nil
}
