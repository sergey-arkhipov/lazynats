package natsclient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// StreamSummary — stream info summary
type StreamSummary struct {
	Name     string
	Subjects []string
	Messages uint64
	Bytes    uint64
}

// ListStreams sorted streams
func (c *Client) ListStreams(ctx context.Context) ([]StreamSummary, error) {
	var result []StreamSummary

	lister := c.js.ListStreams(ctx, jetstream.WithStreamListSubject("*.>"))
	for info := range lister.Info() {
		subjects := make([]string, 0)

		// Get existed subjects
		if info.State.Msgs > 0 {
			stream, err := c.js.Stream(ctx, info.Config.Name)
			if err == nil {
				// WithSubjectFilter(">") — key option; without it, State.Subjects will be empty.
				si, err := stream.Info(ctx, jetstream.WithSubjectFilter(">"))
				if err == nil && len(si.State.Subjects) > 0 {
					for subj := range si.State.Subjects {
						subjects = append(subjects, subj)
					}
					sort.Strings(subjects)
				}
			}
		}
		if strings.HasPrefix(info.Config.Name, "KV_") {
			continue
		}
		result = append(result, StreamSummary{
			Name:     info.Config.Name,
			Subjects: subjects,
			Messages: info.State.Msgs,
			Bytes:    info.State.Bytes,
		})
	}
	if err := lister.Err(); err != nil {
		return nil, fmt.Errorf("list streams: %w", err)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// StreamInfo streams info params
func (c *Client) StreamInfo(ctx context.Context, name string) (*jetstream.StreamInfo, error) {
	stream, err := c.js.Stream(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get stream %q: %w", name, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("info of stream %q: %w", name, err)
	}
	return info, nil
}

// DeleteStream destroy stream
func (c *Client) DeleteStream(ctx context.Context, name string) error {
	if err := c.js.DeleteStream(ctx, name); err != nil {
		return fmt.Errorf("delete stream %q: %w", name, err)
	}
	return nil
}

// CreateStream create stream with default conf - minimum subj and name required
func (c *Client) CreateStream(ctx context.Context, name string, subjects []string) error {
	_, err := c.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
	})
	if err != nil {
		return fmt.Errorf("create stream %q: %w", name, err)
	}
	return nil
}
