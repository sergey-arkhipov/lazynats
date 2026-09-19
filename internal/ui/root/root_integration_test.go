package root

import (
	"context"
	"strings"
	"testing"
	"time"

	"lazynats/internal/ui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func prepareKVBucket(t *testing.T, url, bucket string) {
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

	_, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket})
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
}

// TestBackgroundTabLoadsData reproduces the bug: buckets load in the background
// while the Streams tab is active. After switching, the data is visible without a refresh.
func TestBackgroundTabLoadsData(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "test-bucket")

	client := newTestClient(t, url)
	m := New(client, theme.Default())

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = newM.(Model)

	bucketCmd := m.buckets.Init()
	msg := bucketCmd()

	newM, _ = m.Update(msg)
	rm := newM.(Model)

	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyTab})
	rm = newM.(Model)

	if rm.tab != TabBuckets {
		t.Fatalf("tab = %v, want TabBuckets", rm.tab)
	}

	out := rm.View()
	if !strings.Contains(out, "test-bucket") {
		t.Errorf("View() missing bucket after tab switch.\nGot:\n%s", out)
	}
}

func TestRefreshLoadsCurrentTab(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "refresh-bucket")

	client := newTestClient(t, url)
	m := New(client, theme.Default())
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = newM.(Model)

	m.tab = TabBuckets

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}

	msg := cmd()
	rm := newM.(Model)
	newM, _ = rm.Update(msg)
	rm = newM.(Model)

	out := rm.View()
	if !strings.Contains(out, "refresh-bucket") {
		t.Errorf("View() missing bucket after refresh.\nGot:\n%s", out)
	}
}
