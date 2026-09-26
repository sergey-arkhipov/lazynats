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

func TestRefreshClearsActiveFilterAndReloadsItems(t *testing.T) {
	url := startJetStreamServer(t)
	prepareKVBucket(t, url, "refresh-bucket")
	prepareKVBucket(t, url, "other-bucket")

	client := newTestClient(t, url)
	m := New(client, theme.Default())
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = newM.(Model)
	m.tab = TabBuckets

	// initial load
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = newM.(Model)
	msg := cmd()
	newM, _ = m.Update(msg)
	m = newM.(Model)

	// enter filter mode, type something that matches only one bucket
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newM.(Model)

	for _, r := range "other" {
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(Model)
		m = drainCmd(t, m, cmd)
	}

	// apply the filter
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	if !strings.Contains(m.View(), "other-bucket") {
		t.Fatalf("test setup: expected filtered view to contain other-bucket.\nGot:\n%s", m.View())
	}
	if strings.Contains(m.View(), "refresh-bucket") {
		t.Fatalf("test setup: expected filter to hide refresh-bucket.\nGot:\n%s", m.View())
	}

	// refresh while filter is applied
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	m = newM.(Model)
	msg = cmd()
	newM, _ = m.Update(msg)
	m = newM.(Model)

	out := m.View()
	if !strings.Contains(out, "refresh-bucket") {
		t.Errorf("View() missing refresh-bucket after refresh — filter was not cleared or items were lost.\nGot:\n%s", out)
	}
	if !strings.Contains(out, "other-bucket") {
		t.Errorf("View() missing other-bucket after refresh.\nGot:\n%s", out)
	}
}

// drainCmd runs cmd once (unwrapping a tea.BatchMsg if produced) and feeds
// the resulting message(s) into Update, returning the updated model. It
// does not chase any further command Update returns — components like
// list's filter cursor return an endless chain of tea.Tick blink commands,
// which would make unbounded recursion here.
func drainCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			m2 := c()
			if m2 == nil {
				continue
			}
			newM, _ := m.Update(m2)
			m = newM.(Model)
		}
		return m
	}
	newM, _ := m.Update(msg)
	return newM.(Model)
}
