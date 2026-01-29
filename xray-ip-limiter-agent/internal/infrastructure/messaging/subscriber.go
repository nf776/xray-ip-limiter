package messaging

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"xray-ip-limiter-agent/internal/domain"

	"github.com/nats-io/nats.go"
)

var _ domain.BlockCommandSubscriber = (*NATSSubscriber)(nil)

const (
	blockSubject = "xray-observer-block"
	streamName   = "OBSERVER_STREAM"
)

type NATSSubscriber struct {
	js     nats.JetStreamContext
	nodeID string
}

func NewNATSSubscriber(js nats.JetStreamContext, nodeID string) *NATSSubscriber {
	return &NATSSubscriber{
		js:     js,
		nodeID: nodeID,
	}
}

func (s *NATSSubscriber) Subscribe(ctx context.Context, handler func(domain.BlockCommand)) error {
	durableName := "agent-block-" + s.nodeID

	sub, err := s.js.PullSubscribe(
		blockSubject,
		durableName,
		nats.BindStream(streamName),
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping block command listener")
			return nil
		default:
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					continue
				}
				if err == nats.ErrTimeout {
					continue
				}

				errMsg := err.Error()
				if errMsg == "nats: no responders available for request" ||
					errMsg == "nats: no messages" {
					time.Sleep(2 * time.Second)
					continue
				}

				slog.Error("block command fetch error", slog.String("error", err.Error()))
				time.Sleep(5 * time.Second)
				continue
			}

			for _, msg := range msgs {
				var block domain.BlockCommand
				if err := json.Unmarshal(msg.Data, &block); err != nil {
					slog.Error("failed to unmarshal block command",
						slog.String("error", err.Error()),
						slog.String("data", string(msg.Data)),
					)
					msg.Term()
					continue
				}

				slog.Info("🚫 BLOCK COMMAND RECEIVED", slog.String("user_id", block.UserID))
				handler(block)

				if err := msg.Ack(); err != nil {
					slog.Error("failed to ack block command", slog.String("error", err.Error()))
				}
			}
		}
	}
}
