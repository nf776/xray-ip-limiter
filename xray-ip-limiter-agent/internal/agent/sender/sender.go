package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"xray-ip-limiter-agent/internal/agent/blocker"
	"xray-ip-limiter-agent/internal/agent/models"
	"xray-ip-limiter-agent/internal/config"
	"xray-ip-limiter-agent/internal/utils/logger"

	"github.com/nats-io/nats.go"
)

type NATSClient struct {
	nc       *nats.Conn
	js       nats.JetStreamContext
	nodeID   string
	bufferCh chan models.LogEntry
	blockCh  chan *nats.Msg
	stopCh   chan struct{}
}

func NewNATSClient(cfg config.NATSConfig, nodeID string) (*NATSClient, error) {
	url := fmt.Sprintf("nats://%s", cfg.Url)

	nc, err := nats.Connect(url, nats.Token(cfg.Token), nats.Name(fmt.Sprintf("xray-observer-agent-%s", nodeID)))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create Jetstream context: %w", err)
	}

	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		slog.Error("nats connection error", logger.Err(err))
	})

	nc.SetDisconnectErrHandler(func(_ *nats.Conn, err error) {
		slog.Error("nats disconnect error", logger.Err(err))
	})

	nc.SetClosedHandler(func(_ *nats.Conn) {
		slog.Info("nats connection closed")
	})

	streamConfig := &nats.StreamConfig{
		Name:      "OBSERVER_STREAM",
		Subjects:  []string{"xray-observer-ip", "xray-observer-block"},
		Storage:   nats.FileStorage,
		MaxMsgs:   100000,
		MaxAge:    24 * time.Hour,
		Retention: nats.WorkQueuePolicy,
	}

	_, err = js.AddStream(streamConfig)
	if err != nil {
		_, err = js.UpdateStream(streamConfig)
		if err != nil {
			slog.Debug("stream setup", logger.Err(err))
		}
	}

	durableName := "agent-block-consumer"
	_, _ = js.AddConsumer("OBSERVER_STREAM", &nats.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: "xray-observer-block",
		AckPolicy:     nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverNewPolicy,
		AckWait:       30 * time.Second,
	})

	return &NATSClient{
		nc:       nc,
		js:       js,
		nodeID:   nodeID,
		bufferCh: make(chan models.LogEntry, 1000),
		stopCh:   make(chan struct{}),
	}, nil
}

func (n *NATSClient) Send(entry models.LogEntry) {
	n.bufferCh <- entry
}

func (n *NATSClient) Start(ctx context.Context) error {
	blockSub, err := n.js.PullSubscribe(
		"xray-observer-block",
		"agent-block-consumer",
		nats.BindStream("OBSERVER_STREAM"),
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to block commands: %w", err)
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-n.stopCh:
				return
			case entry := <-n.bufferCh:
				data, err := json.Marshal(entry)
				if err != nil {
					slog.Error("failed to marshal log entry", logger.Err(err))
					continue
				}

				_, err = n.js.Publish("xray-observer-ip", data)
				if err != nil {
					slog.Error("failed to publish IP event", logger.Err(err))
					continue
				}

				// slog.Debug("IP event published",
				// 	slog.String("user", entry.Email),
				// 	slog.Uint64("seq", ack.Sequence),
				// )
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("stopping block command listener")
				return
			case <-n.stopCh:
				return
			default:
				msgs, err := blockSub.Fetch(10, nats.MaxWait(5*time.Second))
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

					slog.Error("block command fetch error", logger.Err(err))
					time.Sleep(5 * time.Second)
					continue
				}

				if len(msgs) == 0 {
					continue
				}

				for _, msg := range msgs {
					var block models.BlockCommand
					if err := json.Unmarshal(msg.Data, &block); err != nil {
						slog.Error("failed to unmarshal block command",
							logger.Err(err),
							slog.String("data", string(msg.Data)),
						)
						msg.Term()
						continue
					}

					slog.Info("🚫 BLOCK COMMAND RECEIVED:", slog.String("user_id", block.UserID))
					blocker.BlockIPs(block)
					if err := msg.Ack(); err != nil {
						slog.Error("failed to ack block command", logger.Err(err))
					}
				}
			}
		}
	})

	<-ctx.Done()
	close(n.bufferCh)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		slog.Warn("Agent shutdown timed out")
	}

	n.nc.Close()
	return nil
}
