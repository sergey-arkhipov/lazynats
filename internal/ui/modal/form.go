package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazynats/internal/ui/theme"
)

// FormSubmittedMsg is sent when the user has confirmed the form
// (by pressing Enter in the last field). Values ​​contains the field values
// in the order specified by FieldSpec, with leading and trailing whitespace removed.
type FormSubmittedMsg struct {
	Values []string
}

// FormCancelledMsg -send by Esc.
type FormCancelledMsg struct{}

// FieldSpec one form field.
type FieldSpec struct {
	Label       string
	Placeholder string
}

type formField struct {
	label string
	input textinput.Model
}

// FormModel — a simple form consisting of several text fields.
// Tab/Shift+Tab (or ↓/↑) switch between fields; pressing Enter in the last field
// submits the form, while Esc cancels it.
type FormModel struct {
	theme  theme.Theme
	title  string
	hint   string
	fields []formField
	focus  int
	errMsg string

	width, height int
}

// NewForm creates a form. The first field immediately receives focus.
func NewForm(th theme.Theme, title, hint string, specs []FieldSpec) FormModel {
	fields := make([]formField, len(specs))
	for i, spec := range specs {
		ti := textinput.New()
		ti.Placeholder = spec.Placeholder
		ti.CharLimit = 512
		ti.Width = 46
		if i == 0 {
			ti.Focus()
		}
		fields[i] = formField{label: spec.Label, input: ti}
	}
	return FormModel{theme: th, title: title, hint: hint, fields: fields}
}

// SetSize sets the size of the screen on which the form is centered.
func (m *FormModel) SetSize(width, height int) { m.width, m.height = width, height }

// SetError displays an error string below the fields (e.g., after a failed
// validation)—the form remains open.
func (m *FormModel) SetError(msg string) { m.errMsg = msg }

// Values - returns the current field values ​​(trimmed of whitespace),
// in the same order as they were passed to NewForm FieldSpec.
func (m FormModel) Values() []string {
	values := make([]string, len(m.fields))
	for i, f := range m.fields {
		values[i] = strings.TrimSpace(f.input.Value())
	}
	return values
}

// Update handles text input and field switching.
func (m FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return FormCancelledMsg{} }

		case "tab", "down":
			m.moveFocus(1)
			return m, nil

		case "shift+tab", "up":
			m.moveFocus(-1)
			return m, nil

		case "enter":
			if m.focus == len(m.fields)-1 {
				return m, func() tea.Msg { return FormSubmittedMsg{Values: m.Values()} }
			}
			m.moveFocus(1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.fields[m.focus].input, cmd = m.fields[m.focus].input.Update(msg)
	return m, cmd
}

func (m *FormModel) moveFocus(delta int) {
	m.fields[m.focus].input.Blur()
	n := len(m.fields)
	m.focus = ((m.focus+delta)%n + n) % n
	m.fields[m.focus].input.Focus()
}

// View renders a centered form over the entire screen.
func (m FormModel) View() string {
	th := m.theme

	rows := []string{th.Title.Render(m.title), ""}
	for _, f := range m.fields {
		rows = append(rows, th.Muted.Render(f.label+":"))
		rows = append(rows, f.input.View())
	}
	if m.errMsg != "" {
		rows = append(rows, "")
		rows = append(rows, th.Danger.Render(m.errMsg))
	}
	rows = append(rows, "")
	rows = append(rows, th.Muted.Render(m.hint))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	box := th.PanelBorderOn.Width(60).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
