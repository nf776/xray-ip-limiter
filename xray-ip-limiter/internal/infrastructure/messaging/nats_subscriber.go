package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"xray-ip-limiter/internal/usecase"

	"github.com/nats-io/nats.go"
)

type NATSSubscriberConfig struct {
	WorkersCount int
	StreamName   string
	Subject      string
	ConsumerName string
}

type NATSSubscriber struct {
	js        nats.JetStreamContext
	cfg       NATSSubscriberConfig
	ipLimiter *usecase.IPLimiter
}

func NewNATSSubscriber(js nats.JetStreamContext, cfg NATSSubscriberConfig, ipLimiter *usecase.IPLimiter) *NATSSubscriber {
	return &NATSSubscriber{
		js:        js,
		cfg:       cfg,
		ipLimiter: ipLimiter,
	}
}

func (s *NATSSubscriber) Start(ctx context.Context) error {
	if err := s.setupStream(); err != nil {
		slog.Warn("stream setup", slog.String("error", err.Error()))
	}

	sub, err := s.js.PullSubscribe(
		s.cfg.Subject,
		s.cfg.ConsumerName,
		nats.BindStream(s.cfg.StreamName),
		nats.DeliverAll(),
		nats.AckExplicit(),
	)
	if err != nil {
		return fmt.Errorf("failed to pull subscribe: %w", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < s.cfg.WorkersCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.worker(ctx, sub)
		}()
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

func (s *NATSSubscriber) setupStream() error {
	streamConfig := &nats.StreamConfig{
		Name:      s.cfg.StreamName,
		Subjects:  []string{"xray-observer-ip", "xray-observer-block"},
		Storage:   nats.FileStorage,
		MaxMsgs:   100000,
		MaxAge:    24 * time.Hour,
		Retention: nats.InterestPolicy,
	}

	_, err := s.js.AddStream(streamConfig)
	if err != nil {
		_, err = s.js.UpdateStream(streamConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *NATSSubscriber) worker(ctx context.Context, sub *nats.Subscription) {
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

				slog.Error("fetch error", slog.String("error", err.Error()))
				continue
			}

			for _, msg := range msgs {
				s.processMessage(ctx, msg)
			}
		}
	}
}

func (s *NATSSubscriber) processMessage(ctx context.Context, msg *nats.Msg) {
	var event usecase.IPEventInput
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Error("error while unmarshal message", slog.String("error", err.Error()))
		msg.Ack()
		return
	}

	_, err := s.ipLimiter.ProcessIPEvent(ctx, event)
	if err != nil {
		slog.Error("error while processing user ip", slog.String("error", err.Error()))
		return
	}

	msg.Ack()
}
