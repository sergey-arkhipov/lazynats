package bucketsview

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
// Helpers for embedd server
// ---------------------------------------------------------------------

func startJetStreamServer(t *testing.T) string {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random free port
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

// prepareKVBucket creates a bucket and (optionally) puts a key into it
// directly via nats.go/jetstream, in order to test natsclient wrappers
// against pre-existing data.
func prepareKVBucket(t *testing.T, url, bucket, key string, value []byte) {
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

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:  bucket,
		History: 5,
	})
	if err != nil {
		t.Fatalf("create kv bucket: %v", err)
	}

	if key != "" {
		if _, err := kv.Put(ctx, key, value); err != nil {
			t.Fatalf("put kv value: %v", err)
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

func TestLoadBucketsCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "bucket-a", "", nil)
	prepareKVBucket(t, url, "bucket-b", "", nil)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadBucketsCmd()()

	loaded, ok := msg.(bucketsLoadedMsg)
	if !ok {
		t.Fatalf("expected bucketsLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if len(loaded.items) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(loaded.items))
	}

	names := make(map[string]bool, len(loaded.items))
	for _, b := range loaded.items {
		names[b.Name] = true
	}
	if !names["bucket-a"] || !names["bucket-b"] {
		t.Errorf("expected bucket-a and bucket-b, got %+v", loaded.items)
	}
}

func TestLoadKeysCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "test-bucket", "key-1", []byte("v1"))
	prepareKVBucket(t, url, "test-bucket", "key-2", []byte("v2"))

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadKeysCmd("test-bucket")()

	loaded, ok := msg.(keysLoadedMsg)
	if !ok {
		t.Fatalf("expected keysLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if loaded.bucket != "test-bucket" {
		t.Errorf("bucket = %q, want %q", loaded.bucket, "test-bucket")
	}
	if len(loaded.items) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(loaded.items))
	}

	got := make(map[string]bool, len(loaded.items))
	for _, k := range loaded.items {
		got[k] = true
	}
	if !got["key-1"] || !got["key-2"] {
		t.Errorf("expected key-1 and key-2, got %v", loaded.items)
	}
}

func TestLoadValueCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	wantValue := []byte("hello-nats")
	prepareKVBucket(t, url, "val-bucket", "val-key", wantValue)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadValueCmd("val-bucket", "val-key")()

	vmsg, ok := msg.(valueLoadedMsg)
	if !ok {
		t.Fatalf("expected valueLoadedMsg, got %T", msg)
	}
	if vmsg.err != nil {
		t.Fatalf("unexpected error: %v", vmsg.err)
	}
	if vmsg.bucket != "val-bucket" {
		t.Errorf("bucket = %q, want %q", vmsg.bucket, "val-bucket")
	}
	if vmsg.key != "val-key" {
		t.Errorf("key = %q, want %q", vmsg.key, "val-key")
	}
	if string(vmsg.entry.value) != string(wantValue) {
		t.Errorf("value = %q, want %q", vmsg.entry.value, wantValue)
	}
	if vmsg.entry.revision == 0 {
		t.Error("revision should be > 0")
	}
	if vmsg.entry.created.IsZero() {
		t.Error("created should not be zero")
	}
	if vmsg.entry.op == "" {
		t.Error("op should not be empty")
	}
}

func TestLoadBucketStatusCmd_Integration(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "status-bucket", "", nil)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadBucketStatusCmd("status-bucket")()

	smsg, ok := msg.(bucketStatusLoadedMsg)
	if !ok {
		t.Fatalf("expected bucketStatusLoadedMsg, got %T", msg)
	}
	if smsg.err != nil {
		t.Fatalf("unexpected error: %v", smsg.err)
	}
	if smsg.bucket != "status-bucket" {
		t.Errorf("bucket = %q, want %q", smsg.bucket, "status-bucket")
	}
	if smsg.status == nil {
		t.Fatal("status should not be nil")
	}
	if smsg.status.Bucket() != "status-bucket" {
		t.Errorf("status.Bucket() = %q, want %q", smsg.status.Bucket(), "status-bucket")
	}
}

// ---------------------------------------------------------------------
// Test for errors
// // ---------------------------------------------------------------------

func TestLoadValueCmd_BucketNotFound(t *testing.T) {
	url := startJetStreamServer(t)
	// do not create bucket

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadValueCmd("ghost-bucket", "ghost-key")()

	vmsg, ok := msg.(valueLoadedMsg)
	if !ok {
		t.Fatalf("expected valueLoadedMsg, got %T", msg)
	}
	if vmsg.err == nil {
		t.Error("expected error for non-existent bucket")
	}
}

func TestLoadKeysCmd_BucketNotFound(t *testing.T) {
	url := startJetStreamServer(t)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadKeysCmd("ghost-bucket")()

	kmsg, ok := msg.(keysLoadedMsg)
	if !ok {
		t.Fatalf("expected keysLoadedMsg, got %T", msg)
	}
	if kmsg.err == nil {
		t.Error("expected error for non-existent bucket")
	}
}

func TestLoadBucketStatusCmd_BucketNotFound(t *testing.T) {
	url := startJetStreamServer(t)

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	msg := m.loadBucketStatusCmd("ghost-bucket")()

	smsg, ok := msg.(bucketStatusLoadedMsg)
	if !ok {
		t.Fatalf("expected bucketStatusLoadedMsg, got %T", msg)
	}
	if smsg.err == nil {
		t.Error("expected error for non-existent bucket")
	}
}
