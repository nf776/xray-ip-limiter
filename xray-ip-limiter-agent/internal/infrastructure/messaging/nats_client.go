package messaging

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSClientConfig struct {
	URL    string
	Token  string
	NodeID string
}

type NATSClient struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	nodeID string
}

func NewNATSClient(cfg NATSClientConfig) (*NATSClient, error) {
	url := fmt.Sprintf("nats://%s", cfg.URL)

	nc, err := nats.Connect(
		url,
		nats.Token(cfg.Token),
		nats.Name(fmt.Sprintf("xray-observer-agent-%s", cfg.NodeID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		slog.Error("nats error", slog.String("error", err.Error()))
	})

	nc.SetDisconnectErrHandler(func(_ *nats.Conn, err error) {
		if err != nil {
			slog.Error("nats disconnect", slog.String("error", err.Error()))
		}
	})

	nc.SetReconnectHandler(func(_ *nats.Conn) {
		slog.Info("nats reconnected")
	})

	nc.SetClosedHandler(func(_ *nats.Conn) {
		slog.Info("nats connection closed")
	})

	if err := setupStream(js); err != nil {
		slog.Debug("stream setup", slog.String("error", err.Error()))
	}

	if err := setupConsumer(js, cfg.NodeID); err != nil {
		slog.Debug("consumer setup", slog.String("error", err.Error()))
	}

	return &NATSClient{
		conn:   nc,
		js:     js,
		nodeID: cfg.NodeID,
	}, nil
}

func (c *NATSClient) JetStream() nats.JetStreamContext {
	return c.js
}

func (c *NATSClient) NodeID() string {
	return c.nodeID
}

func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func setupStream(js nats.JetStreamContext) error {
	streamConfig := &nats.StreamConfig{
		Name:      "OBSERVER_STREAM",
		Subjects:  []string{"xray-observer-ip", "xray-observer-block"},
		Storage:   nats.FileStorage,
		MaxMsgs:   100000,
		MaxAge:    24 * time.Hour,
		Retention: nats.InterestPolicy,
	}

	_, err := js.AddStream(streamConfig)
	if err != nil {
		_, err = js.UpdateStream(streamConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func setupConsumer(js nats.JetStreamContext, nodeID string) error {
	durableName := "agent-block-" + nodeID

	_, err := js.AddConsumer("OBSERVER_STREAM", &nats.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: "xray-observer-block",
		AckPolicy:     nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverNewPolicy,
		AckWait:       30 * time.Second,
	})

	return err
}
