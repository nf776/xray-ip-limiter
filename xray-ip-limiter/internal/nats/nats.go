package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"xray-ip-limiter/internal/config"
	"xray-ip-limiter/internal/models"
	"xray-ip-limiter/internal/processor"
	"xray-ip-limiter/internal/utils/logger"

	"github.com/nats-io/nats.go"
)

type NATSClient struct {
	cfg  config.NATSConfig
	proc *processor.IPProcessor
}

func NewNATSClient(cfg config.NATSConfig, proc *processor.IPProcessor) *NATSClient {
	return &NATSClient{
		cfg:  cfg,
		proc: proc,
	}
}

func (n *NATSClient) Start(ctx context.Context) error {
	workersCount := n.cfg.WorkersCount

	opts := nats.Options{
		Url:            n.cfg.Url,
		Token:          n.cfg.Token,
		Name:           "xray-observer-main",
		AllowReconnect: true,
	}

	nc, err := opts.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to nats: %w", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get jetstream context: %w", err)
	}

	streamConfig := &nats.StreamConfig{
		Name:      "OBSERVER_STREAM",
		Subjects:  []string{"xray-observer-ip", "xray-observer-block"},
		Storage:   nats.FileStorage,
		MaxMsgs:   100000,
		MaxAge:    24 * time.Hour,
		Retention: nats.InterestPolicy,
	}

	_, err = js.AddStream(streamConfig)
	if err != nil {
		_, err = js.UpdateStream(streamConfig)
		if err != nil {
			slog.Warn("stream setup", logger.Err(err))
		}
	}

	sub, err := js.PullSubscribe(
		"xray-observer-ip",
		"observer-cons-1",
		nats.BindStream("OBSERVER_STREAM"),
		nats.DeliverAll(),
		nats.AckExplicit(),
	)
	if err != nil {
		return fmt.Errorf("failed to pull subscribe: %w", err)
	}

	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		slog.Error("nats error", logger.Err(err))
	})

	nc.SetDisconnectErrHandler(func(_ *nats.Conn, err error) {
		slog.Error("nats disconnect error", logger.Err(err))
	})

	nc.SetClosedHandler(func(_ *nats.Conn) {
		slog.Info("nats closed")
	})

	var wg sync.WaitGroup

	for range workersCount {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					msgs, err := sub.Fetch(10, nats.MaxWait(1*time.Second))
					if err != nil {
						if err == nats.ErrTimeout {
							continue
						}

						errMsg := err.Error()
						if errMsg == "nats: no responders available for request" ||
							errMsg == "nats: no messages" {
							time.Sleep(2 * time.Second)
							continue
						}

						slog.Error("fetch error", logger.Err(err))
						continue
					}

					for _, msg := range msgs {
						var data models.UserIPEvent

						err := json.Unmarshal(msg.Data, &data)
						if err != nil {
							slog.Error("error while unmarshal message", logger.Err(err))
							continue
						}
						block, err := n.proc.ProcessUserIP(ctx, data)
						if err != nil {
							slog.Error("error while processing user ip", logger.Err(err))
							continue
						}

						if block != nil {
							blockData, err := json.Marshal(block)
							if err != nil {
								slog.Error("error while marshal block message", logger.Err(err))
								continue
							}

							_, err = js.Publish("xray-observer-block", blockData)
							if err != nil {
								slog.Error("error while sending block message", logger.Err(err))
								continue
							}

							slog.Info(
								"Sending block message to all agents...",
								slog.String("user", block.UserID),
							)
						}
						msg.Ack()
					}
				}
			}
		})
	}

	<-ctx.Done()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		slog.Warn("workers shutdown timed out")
	}

	return nil
}
