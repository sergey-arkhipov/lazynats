package statusbar

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"lazynats/internal/ui/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// TestMain forces a color profile so lipgloss actually emits ANSI codes
// under `go test` (no TTY) instead of silently rendering plain text.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func newTestModel() Model {
	return New(theme.Theme{})
}

// ---------------------------------------------------------------------
// New / setters
// ---------------------------------------------------------------------

func TestNew_StartsDisconnectedWithNoHints(t *testing.T) {
	m := newTestModel()
	if m.connected {
		t.Error("new Model should start disconnected")
	}
	if m.serverURL != "" {
		t.Errorf("new Model serverURL = %q, want empty", m.serverURL)
	}
	if len(m.hints) != 0 {
		t.Errorf("new Model hints = %v, want empty", m.hints)
	}
	if m.width != 0 {
		t.Errorf("new Model width = %d, want 0", m.width)
	}
}

func TestSetWidth(t *testing.T) {
	m := newTestModel()
	m.SetWidth(120)
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

func TestSetConnection(t *testing.T) {
	m := newTestModel()
	m.SetConnection(true, "nats://localhost:4222")
	if !m.connected {
		t.Error("expected connected = true")
	}
	if m.serverURL != "nats://localhost:4222" {
		t.Errorf("serverURL = %q, want %q", m.serverURL, "nats://localhost:4222")
	}

	// Flipping back to disconnected should update, not just latch true.
	m.SetConnection(false, "")
	if m.connected {
		t.Error("expected connected = false after second SetConnection call")
	}
}

func TestSetHints(t *testing.T) {
	m := newTestModel()
	hints := []Hint{{Key: "q", Action: "exit"}, {Key: "r", Action: "refresh"}}
	m.SetHints(hints)
	if len(m.hints) != 2 {
		t.Fatalf("hints len = %d, want 2", len(m.hints))
	}
	if m.hints[0] != hints[0] || m.hints[1] != hints[1] {
		t.Errorf("hints = %v, want %v", m.hints, hints)
	}
}

// ---------------------------------------------------------------------
// View: connection status text
// ---------------------------------------------------------------------

func TestView_OfflineByDefault(t *testing.T) {
	m := newTestModel()
	m.SetWidth(60)
	out := stripANSI(m.View())
	if !strings.Contains(out, "● offline") {
		t.Errorf("expected offline indicator in view, got: %q", out)
	}
}

func TestView_ConnectedShowsServerURL(t *testing.T) {
	m := newTestModel()
	m.SetWidth(60)
	m.SetConnection(true, "nats://localhost:4222")
	out := stripANSI(m.View())
	if !strings.Contains(out, "● nats://localhost:4222") {
		t.Errorf("expected connected indicator with server URL, got: %q", out)
	}
	if strings.Contains(out, "offline") {
		t.Errorf("did not expect 'offline' text while connected, got: %q", out)
	}
}

// ---------------------------------------------------------------------
// View: hints formatting
// ---------------------------------------------------------------------

func TestView_NoHints_NoTrailingSeparator(t *testing.T) {
	m := newTestModel()
	m.SetWidth(60)
	m.SetHints(nil)
	out := stripANSI(m.View())
	if strings.Contains(out, "·") {
		t.Errorf("did not expect a separator with no hints, got: %q", out)
	}
}

func TestView_SingleHint_NoSeparator(t *testing.T) {
	m := newTestModel()
	m.SetWidth(60)
	m.SetHints([]Hint{{Key: "q", Action: "exit"}})
	out := stripANSI(m.View())
	if !strings.Contains(out, "q exit") {
		t.Errorf("expected %q in output, got: %q", "q exit", out)
	}
	if strings.Contains(out, "·") {
		t.Errorf("did not expect a separator with a single hint, got: %q", out)
	}
}

func TestView_MultipleHints_JoinedWithSeparatorInOrder(t *testing.T) {
	m := newTestModel()
	m.SetWidth(80)
	m.SetHints([]Hint{
		{Key: "j/k", Action: "navigation"},
		{Key: "n", Action: "create"},
		{Key: "q", Action: "exit"},
	})
	out := stripANSI(m.View())

	i1 := strings.Index(out, "j/k navigation")
	i2 := strings.Index(out, "n create")
	i3 := strings.Index(out, "q exit")

	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("missing expected hint text in output: %q", out)
	}
	if i1 >= i2 || i2 >= i3 {
		t.Errorf("hints not rendered in order, indices: %d, %d, %d in %q", i1, i2, i3, out)
	}
	if strings.Count(out, "·") != 2 {
		t.Errorf("expected 2 separators for 3 hints, got %d in %q", strings.Count(out, "·"), out)
	}
}

// ---------------------------------------------------------------------
// View: gap calculation between left (status) and right (hints)
// ---------------------------------------------------------------------

// gapBetween finds the literal `left` and `right` substrings in `out`
// and returns the number of characters between them. Assumes left
// appears exactly once before right, which holds for our test fixtures.
func gapBetween(t *testing.T, out, left, right string) int {
	t.Helper()
	li := strings.Index(out, left)
	if li < 0 {
		t.Fatalf("left text %q not found in output %q", left, out)
	}
	ri := strings.Index(out[li+len(left):], right)
	if ri < 0 {
		t.Fatalf("right text %q not found after left text in output %q", right, out)
	}
	return ri
}

func TestView_GapFallsBackToOneWhenWidthTooSmall(t *testing.T) {
	m := newTestModel()
	m.SetWidth(1) // far too small for the content -> raw gap goes negative
	m.SetHints([]Hint{{Key: "q", Action: "exit"}})
	out := stripANSI(m.View())

	gap := gapBetween(t, out, "● offline", "q exit")
	if gap != 1 {
		t.Errorf("gap = %d, want 1 (minimum fallback)", gap)
	}
}

func TestView_GapFallsBackToOneWhenWidthIsZero(t *testing.T) {
	m := newTestModel()
	m.SetWidth(0)
	m.SetHints([]Hint{{Key: "q", Action: "exit"}})
	out := stripANSI(m.View())

	gap := gapBetween(t, out, "● offline", "q exit")
	if gap != 1 {
		t.Errorf("gap = %d, want 1 (minimum fallback)", gap)
	}
}

func TestView_GapGrowsWithWidth(t *testing.T) {
	m := newTestModel()
	m.SetHints([]Hint{{Key: "q", Action: "exit"}})

	m.SetWidth(40)
	outNarrow := stripANSI(m.View())
	gapNarrow := gapBetween(t, outNarrow, "● offline", "q exit")

	m.SetWidth(120)
	outWide := stripANSI(m.View())
	gapWide := gapBetween(t, outWide, "● offline", "q exit")

	if gapWide <= gapNarrow {
		t.Errorf("expected gap to grow with width: gap(40)=%d, gap(120)=%d", gapNarrow, gapWide)
	}
}

func TestView_DoesNotPanicWithEmptyEverything(t *testing.T) {
	m := newTestModel()
	// No SetWidth, no SetConnection, no SetHints called at all.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked on zero-value Model: %v", r)
		}
	}()
	_ = m.View()
}
