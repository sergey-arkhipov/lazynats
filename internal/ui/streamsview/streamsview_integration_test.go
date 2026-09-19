package streamsview

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/theme"
)

// ---------------------------------------------------------------------
// Embedded NATS helpers
// ---------------------------------------------------------------------

func startJetStreamServer(t *testing.T) string {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}

	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create embedded nats server: %v", err)
	}
	srv.Start()
	t.Cleanup(srv.Shutdown)

	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("embedded nats server did not become ready in time")
	}
	return srv.ClientURL()
}

// prepareStream create JetStream and publish msg.
func prepareStream(t *testing.T, url, stream, subject string, messages [][]byte) {
	t.Helper()

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream new: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     stream,
		Subjects: []string{subject},
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	for _, data := range messages {
		if _, err := js.Publish(ctx, subject, data); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
}

func newTestClient(t *testing.T, url string) *natsclient.Client {
	t.Helper()
	client, err := natsclient.Connect(t.Context(), url)
	if err != nil {
		t.Fatalf("natsclient.New(%q): %v", url, err)
	}
	return client
}

// ---------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------

func TestLoadStreamsCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareStream(t, url, "stream-a", "sa.>", nil)
	prepareStream(t, url, "stream-b", "sb.>", nil)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadStreamsCmd()()

	loaded, ok := msg.(streamsLoadedMsg)
	if !ok {
		t.Fatalf("expected streamsLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if len(loaded.items) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(loaded.items))
	}

	names := make(map[string]bool, len(loaded.items))
	for _, s := range loaded.items {
		names[s.Name] = true
	}
	if !names["stream-a"] || !names["stream-b"] {
		t.Errorf("expected stream-a and stream-b, got %+v", loaded.items)
	}
}

func TestLoadMessagesCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareStream(t, url, "orders", "orders.>", [][]byte{
		[]byte(`{"id":1}`),
		[]byte(`{"id":2}`),
	})

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadMessagesCmd("orders", "orders.>")()

	loaded, ok := msg.(messagesLoadedMsg)
	if !ok {
		t.Fatalf("expected messagesLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if loaded.stream != "orders" {
		t.Errorf("stream = %q, want %q", loaded.stream, "orders")
	}
	if loaded.subject != "orders.>" {
		t.Errorf("subject = %q, want %q", loaded.subject, "orders.>")
	}
	if len(loaded.items) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded.items))
	}
}

func TestLoadStreamInfoCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareStream(t, url, "metrics", "metrics.>", nil)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadStreamInfoCmd("metrics")()

	loaded, ok := msg.(streamInfoLoadedMsg)
	if !ok {
		t.Fatalf("expected streamInfoLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if loaded.name != "metrics" {
		t.Errorf("name = %q, want %q", loaded.name, "metrics")
	}
	if loaded.info == nil {
		t.Fatal("expected non-nil info")
	}
	if loaded.info.Config.Name != "metrics" {
		t.Errorf("info.Config.Name = %q, want %q", loaded.info.Config.Name, "metrics")
	}
}

// ---------------------------------------------------------------------
// Tests for errors
// // ---------------------------------------------------------------------

func TestLoadMessagesCmd_StreamNotFound(t *testing.T) {
	url := startJetStreamServer(t)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadMessagesCmd("ghost", "ghost.>")()

	loaded, ok := msg.(messagesLoadedMsg)
	if !ok {
		t.Fatalf("expected messagesLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Error("expected error for non-existent stream")
	}
}

func TestLoadStreamInfoCmd_StreamNotFound(t *testing.T) {
	url := startJetStreamServer(t)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadStreamInfoCmd("ghost")()

	loaded, ok := msg.(streamInfoLoadedMsg)
	if !ok {
		t.Fatalf("expected streamInfoLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Error("expected error for non-existent stream")
	}
}
