package messaging

import (
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type NATSClientConfig struct {
	URL   string
	Token string
	Name  string
}

type NATSClient struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

func NewNATSClient(cfg NATSClientConfig) (*NATSClient, error) {
	opts := nats.Options{
		Url:            cfg.URL,
		Token:          cfg.Token,
		Name:           cfg.Name,
		AllowReconnect: true,
	}

	nc, err := opts.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}

	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		slog.Error("nats error", slog.String("error", err.Error()))
	})

	nc.SetDisconnectErrHandler(func(_ *nats.Conn, err error) {
		if err != nil {
			slog.Error("nats disconnect error", slog.String("error", err.Error()))
		}
	})

	nc.SetReconnectHandler(func(_ *nats.Conn) {
		slog.Info("nats reconnected")
	})

	nc.SetClosedHandler(func(_ *nats.Conn) {
		slog.Info("nats connection closed")
	})

	return &NATSClient{
		conn: nc,
		js:   js,
	}, nil
}

func (c *NATSClient) JetStream() nats.JetStreamContext {
	return c.js
}

func (c *NATSClient) Connection() *nats.Conn {
	return c.conn
}

func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *NATSClient) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}
