// The modal package contains simple full-screen dialogs: a confirmation
// (Confirm) and a form consisting of text fields (Form). Both are rendered
// over the current screen as the sole visible block (without background
// transparency or compositing—this suffices for an MVP, similar to the
// early versions of many "lazy*" tools).
package modal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazynats/internal/ui/theme"
)

// ConfirmResultMsg — confirming result
type ConfirmResultMsg struct {
	Confirmed bool
}

// ConfirmModel — Yes/No dialog.
type ConfirmModel struct {
	theme   theme.Theme
	title   string
	message string
	danger  bool

	width, height int
}

// NewConfirm creates a confirmation dialog. danger=true colors the message
// as a warning about a dangerous action (e.g., deletion).
func NewConfirm(th theme.Theme, title, message string, danger bool) ConfirmModel {
	return ConfirmModel{theme: th, title: title, message: message, danger: danger}
}

// SetSize sets the size of the screen on which the dialog is centered.
func (m *ConfirmModel) SetSize(width, height int) { m.width, m.height = width, height }

// Update handles y/Enter (confirm) and n/Esc (cancel).
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y", "Y", "enter":
		return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }
	case "n", "N", "esc":
		return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }
	}
	return m, nil
}

// View renders a centered dialog over the entire screen.
func (m ConfirmModel) View() string {
	th := m.theme

	msgStyle := th.Base
	if m.danger {
		msgStyle = th.Danger
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		th.Title.Render(m.title),
		"",
		msgStyle.Render(m.message),
		"",
		th.Muted.Render("y / enter — confirm    n / esc — cancel"),
	)

	box := th.PanelBorderOn.Width(60).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
