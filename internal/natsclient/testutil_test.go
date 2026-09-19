package natsclient

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// startJetStreamServer starts an embedded, JetStream-enabled NATS server
// on a random free port for the duration of the test, and shuts it down
// via t.Cleanup.
func startJetStreamServer(t *testing.T) string {
	t.Helper()
	return startServer(t, true)
}

// startPlainServer starts an embedded NATS server WITHOUT JetStream
// enabled. Used to exercise Connect()'s "jetstream account info" error
// path against a real (but JetStream-less) server.
func startPlainServer(t *testing.T) string {
	t.Helper()
	return startServer(t, false)
}

func startServer(t *testing.T, jetStream bool) string {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random free port, avoids collisions between tests
		JetStream: jetStream,
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

// uniqueBucketName returns a KV bucket name that's unique to this test
// (and this run), so tests don't collide on shared state even if a
// server or test file is later reused/parallelized. NATS KV bucket
// names allow letters, digits, dashes and underscores, so the test
// name is sanitized and a nanosecond-resolution suffix is appended.
func uniqueBucketName(t *testing.T, prefix string) string {
	t.Helper()
	safeName := strings.NewReplacer("/", "-", " ", "_").Replace(t.Name())
	return fmt.Sprintf("%s-%s-%d", prefix, safeName, time.Now().UnixNano())
}
