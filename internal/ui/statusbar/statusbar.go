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

// View statusbar render
func (m Model) View() string {
	th := m.theme

	statusText := "● offline"
	statusStyle := th.Danger
	if m.connected {
		statusText = "● " + m.serverURL
		statusStyle = th.Success
	}

	var hints []string
	for _, h := range m.hints {
		hints = append(hints, th.StatusBarKey.Render(h.Key)+" "+th.Muted.Render(h.Action))
	}
	hintsText := strings.Join(hints, th.Muted.Render(" · "))

	left := statusStyle.Render(statusText)
	right := hintsText

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + right
	return th.StatusBar.Padding(0, 1).Render(line)
}
