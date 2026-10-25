package natsclient

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
)

func setupTestClient(t *testing.T) *Client {
	t.Helper()

	s, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		s.Shutdown()
		t.Fatal("nats server did not become ready")
	}
	t.Cleanup(s.Shutdown)

	c, err := Connect(context.Background(), Options{URL: s.ClientURL()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// publish helper
func publish(t *testing.T, c *Client, subject string) {
	t.Helper()
	if err := c.nc.Publish(subject, []byte("test")); err != nil {
		t.Fatalf("publish to %q: %v", subject, err)
	}
}

func TestListStreams(t *testing.T) {
	c := setupTestClient(t)
	ctx := context.Background()

	// empty stream
	if err := c.CreateStream(ctx, "users", []string{"USERS.*"}); err != nil {
		t.Fatalf("create users: %v", err)
	}
	// stream with messages
	if err := c.CreateStream(ctx, "orders", []string{"ORDERS.*"}); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	publish(t, c, "ORDERS.created")
	publish(t, c, "ORDERS.paid")

	// fake KV bucket
	if err := c.CreateStream(ctx, "KV_bucket", []string{"$KV.bucket.>"}); err != nil {
		t.Fatalf("create KV_bucket: %v", err)
	}

	streams, err := c.ListStreams(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(streams) != 2 {
		t.Fatalf("want 2 streams, got %d: %+v", len(streams), streams)
	}
	// sort by name: orders < users
	if streams[0].Name != "orders" || streams[1].Name != "users" {
		t.Errorf("want sorted [orders users], got [%s %s]", streams[0].Name, streams[1].Name)
	}

	if got := streams[0].Subjects; len(got) != 2 || got[0] != "ORDERS.created" || got[1] != "ORDERS.paid" {
		t.Errorf("orders subjects: got %v", got)
	}
	if got := streams[1].Subjects; len(got) != 0 {
		t.Errorf("users subjects: want empty, got %v", got)
	}
	if streams[0].Messages != 2 {
		t.Errorf("orders messages: want 2, got %d", streams[0].Messages)
	}
}

func TestStreamLifecycle(t *testing.T) {
	c := setupTestClient(t)
	ctx := context.Background()

	if err := c.CreateStream(ctx, "events", []string{"events.>"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	info, err := c.StreamInfo(ctx, "events")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Config.Name != "events" {
		t.Errorf("name: want events, got %s", info.Config.Name)
	}

	if err := c.DeleteStream(ctx, "events"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := c.StreamInfo(ctx, "events"); err == nil ||
		!strings.Contains(err.Error(), `get stream "events"`) {
		t.Errorf("want wrapped get-stream error, got %v", err)
	}
}

func TestConsumerList(t *testing.T) {
	c := setupTestClient(t)
	ctx := context.Background()

	if err := c.CreateStream(ctx, "jobs", []string{"jobs.>"}); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	// create consumer
	_, err := c.js.CreateConsumer(ctx, "jobs", jetstream.ConsumerConfig{
		Durable:       "worker1",
		FilterSubject: "jobs.high",
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	consumers, err := c.ConsumerList(ctx, "jobs")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(consumers) != 1 {
		t.Fatalf("want 1 consumer, got %d", len(consumers))
	}

	cn := consumers[0]
	if cn.Name != "worker1" {
		t.Errorf("name: want worker1, got %s", cn.Name)
	}
	if len(cn.FilterSubjects) != 1 || cn.FilterSubjects[0] != "jobs.high" {
		t.Errorf("filters: got %v", cn.FilterSubjects)
	}
	if cn.Last != nil {
		t.Errorf("last: want nil for never-delivered consumer, got %v", cn.Last)
	}

	// not existed stream with get-stream error
	if _, err := c.ConsumerList(ctx, "nope"); err == nil ||
		!strings.Contains(err.Error(), `get stream "nope"`) {
		t.Errorf("want wrapped get-stream error, got %v", err)
	}
}
