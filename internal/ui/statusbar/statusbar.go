// The statusbar package draws the bottom bar: NATS connection status
// and keyboard shortcut hints.
package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazynats/internal/ui/theme"
)

// Model — statusbar
type Model struct {
	theme     theme.Theme
	width     int
	connected bool
	serverURL string
	hints     []Hint
}

// Hint — one hint of the "key - action" type.
type Hint struct {
	Key    string
	Action string
}

// New create statusbar
func New(th theme.Theme) Model {
	return Model{theme: th}
}

// SetWidth for statusbar
func (m *Model) SetWidth(w int) { m.width = w }

// SetConnection refresh connection status
func (m *Model) SetConnection(connected bool, serverURL string) {
	m.connected = connected
	m.serverURL = serverURL
}

// SetHints sets the list of key hints for the current context.
func (m *Model) SetHints(hints []Hint) { m.hints = hints }

// View renders the status bar, wrapping hints onto a second line if they
// don't fit in a single row.
func (m Model) View() string {
	th := m.theme

	statusText := "● offline"
	statusStyle := th.Danger
	if m.connected {
		statusText = "● " + m.serverURL
		statusStyle = th.Success
	}
	left := statusStyle.Render(statusText)
	leftW := lipgloss.Width(left)

	// Account for the frame padding (1 left + 1 right).
	avail := max(m.width-2, 1)

	sep := th.Muted.Render(" · ")
	sepW := lipgloss.Width(" · ")

	// Split hints into lines that each fit within avail. The first line
	// also accounts for the status text plus a single separating space.
	var lines []string
	var cur []string
	curW := 0

	// Hints are never truncated: a hint wider than the available row
	// overflows and the terminal clips it, which reads better than a
	// word cut off mid-way.
	for _, h := range m.hints {
		hint := th.StatusBarKey.Render(h.Key) + " " + th.Muted.Render(h.Action)
		hW := lipgloss.Width(hint)

		budget := lineBudget(avail, leftW, len(lines) == 0)
		extra := hW
		if len(cur) > 0 {
			extra += sepW
		}

		switch {
		case curW > 0 && curW+extra > budget:
			// Current line is full — push the rest onto a new line.
			lines = append(lines, strings.Join(cur, sep))
			cur, curW = nil, 0
			extra = hW
		case curW == 0 && len(lines) == 0 && hW > budget && hW <= avail:
			// The first hint doesn't fit next to the status text, but
			// fits on its own line — don't glue it to the status.
			lines = append(lines, "")
		}

		cur = append(cur, hint)
		curW += extra
	}
	// Always flush: with no hints at all, lines[0] must still exist.
	if len(cur) > 0 || len(lines) == 0 {
		lines = append(lines, strings.Join(cur, sep))
	}

	// First row: status + gap + hints, with a minimum one-space gap.
	firstRow := left
	if hints := lines[0]; hints != "" {
		gap := max(avail-leftW-lipgloss.Width(hints), 1)
		firstRow += strings.Repeat(" ", gap) + hints
	}

	rows := make([]string, 0, len(lines))
	rows = append(rows, th.StatusBar.Padding(0, 1).Render(firstRow))
	for _, r := range lines[1:] {
		rows = append(rows, th.StatusBar.Padding(0, 1).Render(r))
	}

	return strings.Join(rows, "\n")
}

// lineBudget returns the available width for hints on the given line; the
// first line shares the row with the status text and a one-space gap.
func lineBudget(avail, leftW int, first bool) int {
	if !first {
		return avail
	}
	return max(avail-leftW-1, 0)
}
