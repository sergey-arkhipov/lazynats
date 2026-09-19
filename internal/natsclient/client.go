// Package natsclient — connect to nats server and return Client for UI
package natsclient

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client for app
type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// Connect to nats server
func Connect(ctx context.Context, url string) (*Client, error) {
	nc, err := nats.Connect(
		url,
		nats.Name("lazynats"),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats %q: %w", url, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("init jetstream: %w", err)
	}

	c := &Client{nc: nc, js: js}

	// Check JetStream enabled?
	if _, err := js.AccountInfo(ctx); err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream account info: %w", err)
	}

	return c, nil
}

// Close connection
func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

// Status connection (for statusbar)
func (c *Client) Status() nats.Status {
	if c.nc == nil {
		return nats.CLOSED
	}
	return c.nc.Status()
}

// ConnectedURL - current server URL
func (c *Client) ConnectedURL() string {
	if c.nc == nil {
		return ""
	}
	return c.nc.ConnectedUrl()
}
