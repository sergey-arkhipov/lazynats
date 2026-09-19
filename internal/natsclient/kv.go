package natsclient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// BucketSummary — KV summary for UI
type BucketSummary struct {
	Name   string
	Keys   int    // uniq keys
	Values uint64 // KV entry count
	Bytes  uint64 // size in bytes
}

// ListBuckets sorted KV list
func (c *Client) ListBuckets(ctx context.Context) ([]BucketSummary, error) {
	var result []BucketSummary

	lister := c.js.ListStreams(ctx, jetstream.WithStreamListSubject("$KV.>"))
	for info := range lister.Info() {
		kvName := strings.TrimPrefix(info.Config.Name, "KV_")

		kv, err := c.js.KeyValue(ctx, kvName)
		if err != nil {
			continue
		}

		// Count keys
		keyLister, err := kv.ListKeys(ctx)
		keyCount := 0
		if err == nil {
			for range keyLister.Keys() {
				keyCount++
			}
		}

		// metrics
		status, err := kv.Status(ctx)
		var values, bytes uint64
		if err == nil {
			values = status.Values()
			bytes = status.Bytes()
		}

		result = append(result, BucketSummary{
			Name:   kvName,
			Keys:   keyCount,
			Values: values,
			Bytes:  bytes,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// BucketKeys list keys for KV
func (c *Client) BucketKeys(ctx context.Context, bucket string) ([]string, error) {
	kv, err := c.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("open bucket %q: %w", bucket, err)
	}

	lister, err := kv.ListKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list keys in %q: %w", bucket, err)
	}

	var keys []string
	for k := range lister.Keys() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// KeyValue for key
func (c *Client) KeyValue(ctx context.Context, bucket, key string) (jetstream.KeyValueEntry, error) {
	kv, err := c.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("open bucket %q: %w", bucket, err)
	}
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get key %q in %q: %w", key, bucket, err)
	}
	return entry, nil
}

// DeleteBucket destroy KV
func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	if err := c.js.DeleteKeyValue(ctx, bucket); err != nil {
		return fmt.Errorf("delete bucket %q: %w", bucket, err)
	}
	return nil
}

// CreateBucket created KV with most defaults
func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	_, err := c.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: bucket,
	})
	if err != nil {
		return fmt.Errorf("create bucket %q: %w", bucket, err)
	}
	return nil
}

// BucketStatus bucket info
func (c *Client) BucketStatus(ctx context.Context, bucket string) (jetstream.KeyValueStatus, error) {
	kv, err := c.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("open bucket %q: %w", bucket, err)
	}
	status, err := kv.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("status of bucket %q: %w", bucket, err)
	}
	return status, nil
}
