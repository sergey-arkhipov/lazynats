package natsclient

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func createStream(t *testing.T, url, name string, retention jetstream.RetentionPolicy, subjects ...string) {
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
		Name:      name,
		Subjects:  subjects,
		Retention: retention,
	})
	if err != nil {
		t.Fatalf("create stream %q: %v", name, err)
	}
}

func publishMessages(t *testing.T, url, subject string, payloads ...[]byte) {
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

	for _, p := range payloads {
		if _, err := js.Publish(ctx, subject, p); err != nil {
			t.Fatalf("publish to %q: %v", subject, err)
		}
	}
}

// ---------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------

func TestFetchLastMessages_LimitsPolicy(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "orders", jetstream.LimitsPolicy, "orders.>")
	publishMessages(t, url, "orders.created",
		[]byte(`1`), []byte(`2`), []byte(`3`), []byte(`4`), []byte(`5`),
	)

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "orders", "orders.>", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	seqs := make([]uint64, len(msgs))
	for i, m := range msgs {
		seqs[i] = m.Sequence
	}
	want := []uint64{3, 4, 5}
	if !slices.Equal(seqs, want) {
		t.Errorf("sequences = %v, want %v", seqs, want)
	}
}

func TestFetchLastMessages_WorkQueuePolicy(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "tasks", jetstream.WorkQueuePolicy, "tasks.>")
	publishMessages(t, url, "tasks.run",
		[]byte(`a`), []byte(`b`), []byte(`c`),
	)

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "tasks", "tasks.>", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	seqs := make([]uint64, len(msgs))
	for i, m := range msgs {
		seqs[i] = m.Sequence
	}
	want := []uint64{2, 3}
	if !slices.Equal(seqs, want) {
		t.Errorf("sequences = %v, want %v", seqs, want)
	}
}

func TestFetchLastMessages_DefaultLimit(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "events", jetstream.LimitsPolicy, "events.>")

	var payloads [][]byte
	for i := 0; i < 60; i++ {
		payloads = append(payloads, []byte(fmt.Sprintf("%d", i)))
	}
	publishMessages(t, url, "events.all", payloads...)

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "events", "events.>", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 50 {
		t.Fatalf("expected default limit 50, got %d", len(msgs))
	}
	if msgs[0].Sequence != 11 {
		t.Errorf("first sequence = %d, want 11", msgs[0].Sequence)
	}
	if msgs[49].Sequence != 60 {
		t.Errorf("last sequence = %d, want 60", msgs[49].Sequence)
	}
}

func TestFetchLastMessages_EmptyStream(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "empty", jetstream.LimitsPolicy, "empty.>")

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "empty", "empty.>", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for empty stream, got %d", len(msgs))
	}
}

func TestFetchLastMessages_SubjectFilterWildcard(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "multi", jetstream.LimitsPolicy, "foo.>", "bar.>")
	publishMessages(t, url, "foo.a", []byte(`1`), []byte(`2`))
	publishMessages(t, url, "foo.b", []byte(`3`))
	publishMessages(t, url, "bar.a", []byte(`4`))

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "multi", "foo.>", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages matching foo.>, got %d", len(msgs))
	}
	for _, m := range msgs {
		if !strings.HasPrefix(m.Subject, "foo.") {
			t.Errorf("unexpected subject %q, want foo.*", m.Subject)
		}
	}
}

func TestFetchLastMessages_LimitGreaterThanTotal(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "small", jetstream.LimitsPolicy, "small.>")
	publishMessages(t, url, "small.x", []byte(`1`), []byte(`2`))

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "small", "small.>", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (limit > total), got %d", len(msgs))
	}
}

func TestFetchLastMessages_StreamNotFound(t *testing.T) {
	url := startJetStreamServer(t)

	client := newTestClient(t, url)
	_, err := client.FetchLastMessages(context.Background(), "ghost", "ghost.>", 10)
	if err == nil {
		t.Fatal("expected error for non-existent stream")
	}
}

func TestFetchLastMessages_InterestPolicy(t *testing.T) {
	url := startJetStreamServer(t)
	createStream(t, url, "interest", jetstream.InterestPolicy, "interest.>")

	// For InterestPolicy, messages are stored only if there is an active
	// consumer at the time of publication. We create a dummy durable before publishing.
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

	stream, err := js.Stream(ctx, "interest")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}

	_, err = stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "hold",
		FilterSubject: "interest.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("create dummy consumer: %v", err)
	}

	publishMessages(t, url, "interest.x", []byte(`1`), []byte(`2`), []byte(`3`))

	client := newTestClient(t, url)
	msgs, err := client.FetchLastMessages(context.Background(), "interest", "interest.>", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	seqs := make([]uint64, len(msgs))
	for i, m := range msgs {
		seqs[i] = m.Sequence
	}
	want := []uint64{2, 3}
	if !slices.Equal(seqs, want) {
		t.Errorf("sequences = %v, want %v", seqs, want)
	}
}
