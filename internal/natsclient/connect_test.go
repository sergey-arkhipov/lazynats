package natsclient

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// ---------------------------------------------------------------------
// Connect
// ---------------------------------------------------------------------

func TestConnect_Success(t *testing.T) {
	url := startJetStreamServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer c.Close()

	if c.Status() != nats.CONNECTED {
		t.Errorf("Status() = %v, want %v", c.Status(), nats.CONNECTED)
	}
	if c.ConnectedURL() == "" {
		t.Error("ConnectedURL() is empty after a successful connect")
	}
}

func TestConnect_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Connect(ctx, "not a valid url")
	if err == nil {
		t.Fatal("expected an error connecting to an invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "connect to nats") {
		t.Errorf("error = %q, want it to mention %q", err.Error(), "connect to nats")
	}
}

func TestConnect_JetStreamNotEnabled(t *testing.T) {
	// Real server, real connection — but JetStream is off, so Connect()
	// should fail on its own account-info check and clean up after
	// itself (close nc) rather than leaking a live connection.
	url := startPlainServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Connect(ctx, url)
	if err == nil {
		t.Fatal("expected an error connecting to a server without JetStream enabled, got nil")
	}
	if !strings.Contains(err.Error(), "jetstream account info") {
		t.Errorf("error = %q, want it to mention %q", err.Error(), "jetstream account info")
	}
}

func TestConnect_ContextCancelledBeforeAccountInfo(t *testing.T) {
	// AccountInfo is the one call in Connect() that actually uses ctx;
	// an already-cancelled context should make Connect() fail there
	// even though the underlying nc.Connect() (which ignores ctx)
	// would otherwise succeed.
	url := startJetStreamServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before Connect() is even called

	_, err := Connect(ctx, url)
	if err == nil {
		t.Fatal("expected an error from Connect() with an already-cancelled context")
	}
}

// ---------------------------------------------------------------------
// Close / Status / ConnectedURL
// ---------------------------------------------------------------------

func TestClient_Close_ClosesConnection(t *testing.T) {
	url := startJetStreamServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	c.Close()

	if c.Status() != nats.CLOSED {
		t.Errorf("Status() after Close() = %v, want %v", c.Status(), nats.CLOSED)
	}
}

func TestClient_Close_IsIdempotent(t *testing.T) {
	url := startJetStreamServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("calling Close() twice panicked: %v", r)
		}
	}()
	c.Close()
	c.Close()
}

func TestClient_Close_NilConnDoesNotPanic(t *testing.T) {
	c := &Client{} // zero-value, nc == nil — as if Connect() never succeeded
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close() panicked on a zero-value Client: %v", r)
		}
	}()
	c.Close()
}

func TestClient_Status_ZeroValueClient(t *testing.T) {
	c := &Client{}
	if got := c.Status(); got != nats.CLOSED {
		t.Errorf("Status() on a zero-value Client = %v, want %v", got, nats.CLOSED)
	}
}

func TestClient_ConnectedURL_ZeroValueClient(t *testing.T) {
	c := &Client{}
	if got := c.ConnectedURL(); got != "" {
		t.Errorf("ConnectedURL() on a zero-value Client = %q, want empty", got)
	}
}
