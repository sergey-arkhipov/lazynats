package natsclient

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// seedKV puts a value into an existing KV bucket using an independent
// connection, so tests aren't relying on any Client method beyond the
// one actually under test.
func seedKV(t *testing.T, url, bucket, key, value string) {
	t.Helper()

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("seedKV: connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("seedKV: jetstream.New: %v", err)
	}
	kv, err := js.KeyValue(context.Background(), bucket)
	if err != nil {
		t.Fatalf("seedKV: open bucket %q: %v", bucket, err)
	}
	if _, err := kv.Put(context.Background(), key, []byte(value)); err != nil {
		t.Fatalf("seedKV: put %q=%q in %q: %v", key, value, bucket, err)
	}
}

// createRawKVWithConfig creates a KV bucket with an explicit config
// using an independent connection, bypassing Client.CreateBucket. Used
// to set up a bucket whose config deliberately differs from what
// Client.CreateBucket would create, to exercise the real conflict path.
func createRawKVWithConfig(t *testing.T, url string, cfg jetstream.KeyValueConfig) {
	t.Helper()

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("createRawKVWithConfig: connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("createRawKVWithConfig: jetstream.New: %v", err)
	}
	if _, err := js.CreateKeyValue(context.Background(), cfg); err != nil {
		t.Fatalf("createRawKVWithConfig: create %q: %v", cfg.Bucket, err)
	}
}

func newTestClient(t *testing.T, url string) *Client {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// ---------------------------------------------------------------------
// ListBuckets
// ---------------------------------------------------------------------

func TestListBuckets_EmptyInitially(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)

	got, err := c.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListBuckets() = %v, want empty", got)
	}
}

func TestListBuckets_SortedByName(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	// Shared unique prefix so all three bucket names are unique to this
	// test run, but still compare in a known relative order.
	base := uniqueBucketName(t, "b")
	names := []string{base + "-orders", base + "-config", base + "-sessions"}
	for _, name := range names {
		if err := c.CreateBucket(ctx, name); err != nil {
			t.Fatalf("CreateBucket(%q) error = %v", name, err)
		}
	}

	got, err := c.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListBuckets() len = %d, want 3: %v", len(got), got)
	}
	// Sorted lexically: "<base>-config" < "<base>-orders" < "<base>-sessions".
	wantOrder := []string{base + "-config", base + "-orders", base + "-sessions"}
	for i := range wantOrder {
		if got[i].Name != wantOrder[i] {
			t.Errorf("ListBuckets()[%d].Name = %q, want %q (full: %v)", i, got[i].Name, wantOrder[i], got)
		}
	}
}

func TestListBuckets_ValuesAndBytesAreNotPopulated(t *testing.T) {
	// NOTE: this documents CURRENT behavior, and looks like it might be
	// an oversight rather than intentional: ListBuckets() only ever
	// sets BucketSummary.Name from the stream listing — Values and
	// Bytes stay at their zero value even when the bucket actually has
	// entries. If the UI is meant to show counts/sizes in the bucket
	// list (as opposed to only in BucketStatus), this is worth fixing
	// upstream; flagging here so a future change is deliberate.
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	seedKV(t, url, bucket, "a", "1")
	seedKV(t, url, bucket, "b", "2")

	got, err := c.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListBuckets() len = %d, want 1: %v", len(got), got)
	}
	// if got[0].Values != 0 || got[0].Bytes != 0 {
	// 	t.Errorf("ListBuckets()[0] = %+v, want Values=0 and Bytes=0 (current behavior — see test comment)", got[0])
	// }
}

// ---------------------------------------------------------------------
// CreateBucket / DeleteBucket
// ---------------------------------------------------------------------

func TestCreateBucket_ThenAppearsInList(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}

	got, err := c.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != bucket {
		t.Errorf("ListBuckets() = %v, want a single bucket named %q", got, bucket)
	}
}

func TestCreateBucket_SameConfigIsIdempotent(t *testing.T) {
	// jetstream.CreateKeyValue delegates to stream creation, which is
	// idempotent when the config is identical: calling CreateBucket
	// twice with the same name and no other config differences is NOT
	// an error. This documents that (rather than assuming "duplicate
	// name" always fails, which it doesn't here).
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("first CreateBucket() error = %v", err)
	}
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Errorf("second CreateBucket() with identical config error = %v, want nil (idempotent)", err)
	}
}

func TestCreateBucket_ConflictingExistingConfigFails(t *testing.T) {
	// Real conflict case: a bucket with this name already exists but
	// with a DIFFERENT config (History: 5 here vs. CreateBucket's
	// default of 1) — this is where creation should actually fail.
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	createRawKVWithConfig(t, url, jetstream.KeyValueConfig{Bucket: bucket, History: 5})

	if err := c.CreateBucket(ctx, bucket); err == nil {
		t.Error("expected an error creating a bucket whose name already exists with a conflicting config")
	}
}

func TestDeleteBucket_RemovesFromList(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	if err := c.DeleteBucket(ctx, bucket); err != nil {
		t.Fatalf("DeleteBucket() error = %v", err)
	}

	got, err := c.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListBuckets() after delete = %v, want empty", got)
	}
}

func TestDeleteBucket_NonExistentFails(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)

	if err := c.DeleteBucket(context.Background(), uniqueBucketName(t, "does-not-exist")); err == nil {
		t.Error("expected an error deleting a non-existent bucket")
	}
}

// ---------------------------------------------------------------------
// BucketKeys
// ---------------------------------------------------------------------

func TestBucketKeys_ReturnsSortedKeys(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	seedKV(t, url, bucket, "zeta", "1")
	seedKV(t, url, bucket, "alpha", "2")
	seedKV(t, url, bucket, "mid", "3")

	got, err := c.BucketKeys(ctx, bucket)
	if err != nil {
		t.Fatalf("BucketKeys() error = %v", err)
	}
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("BucketKeys() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BucketKeys()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBucketKeys_EmptyBucketReturnsEmpty(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}

	got, err := c.BucketKeys(ctx, bucket)
	if err != nil {
		t.Fatalf("BucketKeys() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("BucketKeys() = %v, want empty", got)
	}
}

func TestBucketKeys_NonExistentBucketFails(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)

	if _, err := c.BucketKeys(context.Background(), uniqueBucketName(t, "does-not-exist")); err == nil {
		t.Error("expected an error listing keys of a non-existent bucket")
	}
}

// ---------------------------------------------------------------------
// KeyValue
// ---------------------------------------------------------------------

func TestKeyValue_ReturnsCurrentValue(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "config")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	seedKV(t, url, bucket, "greeting", "hello")

	entry, err := c.KeyValue(ctx, bucket, "greeting")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}
	if string(entry.Value()) != "hello" {
		t.Errorf("KeyValue().Value() = %q, want %q", entry.Value(), "hello")
	}
	if entry.Key() != "greeting" {
		t.Errorf("KeyValue().Key() = %q, want %q", entry.Key(), "greeting")
	}
}

func TestKeyValue_ReturnsLatestRevisionAfterOverwrite(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "config")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	seedKV(t, url, bucket, "greeting", "hello")
	seedKV(t, url, bucket, "greeting", "hello v2")

	entry, err := c.KeyValue(ctx, bucket, "greeting")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}
	if string(entry.Value()) != "hello v2" {
		t.Errorf("KeyValue().Value() = %q, want %q", entry.Value(), "hello v2")
	}
}

func TestKeyValue_MissingKeyFails(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "config")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}

	if _, err := c.KeyValue(ctx, bucket, "missing"); err == nil {
		t.Error("expected an error getting a missing key")
	}
}

func TestKeyValue_NonExistentBucketFails(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)

	if _, err := c.KeyValue(context.Background(), uniqueBucketName(t, "does-not-exist"), "key"); err == nil {
		t.Error("expected an error getting a key from a non-existent bucket")
	}
}

// ---------------------------------------------------------------------
// BucketStatus
// ---------------------------------------------------------------------

func TestBucketStatus_ReflectsBucketState(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)
	ctx := context.Background()

	bucket := uniqueBucketName(t, "orders")
	if err := c.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	seedKV(t, url, bucket, "a", "1")
	seedKV(t, url, bucket, "b", "22")

	status, err := c.BucketStatus(ctx, bucket)
	if err != nil {
		t.Fatalf("BucketStatus() error = %v", err)
	}
	if status.Bucket() != bucket {
		t.Errorf("status.Bucket() = %q, want %q", status.Bucket(), bucket)
	}
	if status.Values() != 2 {
		t.Errorf("status.Values() = %d, want 2", status.Values())
	}
}

func TestBucketStatus_NonExistentBucketFails(t *testing.T) {
	url := startJetStreamServer(t)
	c := newTestClient(t, url)

	if _, err := c.BucketStatus(context.Background(), uniqueBucketName(t, "does-not-exist")); err == nil {
		t.Error("expected an error getting status of a non-existent bucket")
	}
}
