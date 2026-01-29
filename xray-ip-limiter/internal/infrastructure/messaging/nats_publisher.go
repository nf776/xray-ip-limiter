package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"xray-ip-limiter/internal/usecase"

	"github.com/nats-io/nats.go"
)

var _ usecase.BlockPublisher = (*NATSPublisher)(nil)

type NATSPublisher struct {
	js      nats.JetStreamContext
	subject string
}

func NewNATSPublisher(js nats.JetStreamContext, subject string) *NATSPublisher {
	return &NATSPublisher{
		js:      js,
		subject: subject,
	}
}

func (p *NATSPublisher) PublishBlock(ctx context.Context, cmd usecase.BlockCommandOutput) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal block command: %w", err)
	}

	_, err = p.js.Publish(p.subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish block command: %w", err)
	}

	return nil
}
