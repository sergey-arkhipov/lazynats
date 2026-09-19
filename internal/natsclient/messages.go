package natsclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Message from streams
type Message struct {
	Sequence  uint64
	Subject   string
	Timestamp time.Time
	Data      []byte
}

// FetchLastMessages retrieves up to `limit` recent stream messages
// matching `subjectFilter` (which can be either a literal subject
// or a wildcard pattern). It is implemented using an ephemeral pull consumer
// that starts at the calculated offset (LastSeq - limit).
func (c *Client) FetchLastMessages(ctx context.Context, streamName, subjectFilter string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	stream, err := c.js.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("get stream %q: %w", streamName, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("info of stream %q: %w", streamName, err)
	}

	if info.State.Msgs == 0 {
		return nil, nil
	}

	// Count seq range
	startSeq := info.State.FirstSeq
	if info.State.LastSeq > uint64(limit) {
		candidate := info.State.LastSeq - uint64(limit) + 1
		if candidate > startSeq {
			startSeq = candidate
		}
	}

	// WorkQueue: Do not use a consumer without an ack — it will block messages for services.
	// Read directly via DirectGet.
	if info.Config.Retention == jetstream.WorkQueuePolicy {
		var messages []Message
		for seq := startSeq; seq <= info.State.LastSeq; seq++ {
			var opts []jetstream.GetMsgOpt
			if subjectFilter != "" {
				opts = append(opts, jetstream.WithGetMsgSubject(subjectFilter))
			}

			raw, err := stream.GetMsg(ctx, seq, opts...)
			if err != nil {
				if errors.Is(err, jetstream.ErrMsgNotFound) {
					continue
				}
				slog.Error("direct get failed", "seq", seq, "error", err)
				continue
			}

			messages = append(messages, Message{
				Sequence:  raw.Sequence,
				Subject:   raw.Subject,
				Timestamp: raw.Time,
				Data:      raw.Data,
			})
		}
		return messages, nil
	}

	// Limits / Interest: ephemeral consumer with AckNone — safe, doesn't mess with messages.
	// ← FIXED: passing startSeq so we read from the desired position rather than the beginning.
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject:     subjectFilter,
		DeliverPolicy:     jetstream.DeliverByStartSequencePolicy,
		OptStartSeq:       startSeq,
		AckPolicy:         jetstream.AckNonePolicy,
		InactiveThreshold: 2 * time.Minute,
	})
	if err != nil {
		slog.Error("create consumer", "subjectFilter", subjectFilter, "err", err)
		return nil, fmt.Errorf("create ephemeral consumer for %q: %w", subjectFilter, err)
	}
	defer func() {
		_ = stream.DeleteConsumer(context.Background(), cons.CachedInfo().Name)
	}()

	batch, err := cons.FetchNoWait(limit)
	if err != nil {
		return nil, fmt.Errorf("fetch messages for %q: %w", subjectFilter, err)
	}

	var messages []Message
	for msg := range batch.Messages() {
		meta, err := msg.Metadata()
		if err != nil {
			continue
		}
		messages = append(messages, Message{
			Sequence:  meta.Sequence.Stream,
			Subject:   msg.Subject(),
			Timestamp: meta.Timestamp,
			Data:      msg.Data(),
		})
	}
	if err := batch.Error(); err != nil {
		return messages, fmt.Errorf("read message batch for %q: %w", subjectFilter, err)
	}

	return messages, nil
}
