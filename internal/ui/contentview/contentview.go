// Package contentview — the content viewing panel (subject's messages
// or a key's value). Replaces the right-hand list panel on Enter,
// Esc returns back to the list. Text is plain and mouse-selectable
// in the terminal as-is; additionally, "y" copies the whole content
// to the system clipboard.
package contentview

import (
	"strings"
	"unicode"

	"lazynats/internal/ui/theme"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wrap"
)

// CopyResultMsg — the result of an attempt to copy content to the
// clipboard, used to show a status to the user (e.g. in a status line).
type CopyResultMsg struct {
	Err error
}

// Model — state of the viewing panel.
type Model struct {
	theme theme.Theme

	title    string // subject / key — context of what is being viewed
	subtitle string // extra info: seq/time/revision, etc.
	body     string // raw text for display and copying
	notice   string // short status of the last action (e.g. copying)

	vp     viewport.Model
	width  int
	height int
}

// New creates an empty viewing panel.
func New(th theme.Theme) Model {
	return Model{
		theme: th,
		vp:    viewport.New(0, 0),
	}
}

// recalcViewportHeight recomputes how much vertical space the header
// block takes (which depends on title/subtitle and can wrap at narrow
// widths) and resizes the viewport to fill what's left. Must run after
// any change to title, subtitle, notice, or the panel's width/height —
// otherwise a header that grows (e.g. a longer subtitle after messages
// load) leaves the viewport sized for the old, shorter header, and the
// total rendered height overflows the terminal.
func (m *Model) recalcViewportHeight() {
	headerHeight := lipgloss.Height(m.headerBlock()) + 1
	vpHeight := m.height - headerHeight
	vpHeight = max(vpHeight, 0)
	m.vp.Width = m.width
	m.vp.Height = vpHeight
}

// SetContent [TODO:description]
func (m *Model) SetContent(title, subtitle, body string) {
	m.title = title
	m.subtitle = subtitle
	m.body = body
	m.notice = ""
	m.recalcViewportHeight()
	m.setWrappedContent()
	m.vp.GotoTop()
}

// SetSize [TODO:description]
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.recalcViewportHeight()
	if m.body != "" {
		m.setWrappedContent()
	}
}

func (m *Model) setWrappedContent() {
	content := highlightMixedContent(m.body)
	if m.width > 0 {
		m.vp.SetContent(hardWrap(content, m.width))
	} else {
		m.vp.SetContent(content)
	}
}

// Body returns the raw content — what goes to the clipboard on "y".
func (m Model) Body() string { return m.body }

// Update handles scrolling and copying. Esc is handled by the
// parent model (streamsview/bucketsview) — it decides when to
// leave viewing mode and go back to the list.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "y" {
			body := ansi.Strip(m.body)
			return m, func() tea.Msg {
				return CopyResultMsg{Err: clipboard.WriteAll(body)}
			}
		}
	case CopyResultMsg:
		if msg.Err != nil {
			m.notice = "copy failed: " + msg.Err.Error()
		} else {
			m.notice = "copied to clipboard"
		}
		m.recalcViewportHeight()
		return m, nil
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// headerBlock renders the title/subtitle/notice block, wrapped to
// m.width the same way the body content is wrapped, so it never
// exceeds the available terminal width.
func (m Model) headerBlock() string {
	th := m.theme

	titleLine := th.Title.Render(m.title)

	var metaLine string
	if m.subtitle != "" {
		metaLine = th.Muted.Render(m.subtitle)
	}
	if m.notice != "" {
		if metaLine != "" {
			metaLine += "  "
		}
		metaLine += th.Success.Render(m.notice)
	}

	block := titleLine
	if metaLine != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, titleLine, metaLine)
	}

	if m.width > 0 {
		block = hardWrap(block, m.width)
	}

	return block
}

// View renders the header and the scrollable content.
func (m Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerBlock(),
		m.theme.Muted.Render(dividerLine(m.width)),
		m.vp.View(),
	)
}

// SetBytes [TODO:description]
func (m *Model) SetBytes(title, subtitle string, data []byte) {
	m.SetContent(title, subtitle, string(PrettifyJSON(data)))
}

func dividerLine(width int) string {
	if width <= 0 {
		width = 1
	}
	line := make([]byte, width)
	for i := range line {
		line[i] = '-'
	}
	return string(line)
}

func highlightMixedContent(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if isJSONLine(trimmed) {
			lines[i] = highlightJSONLine(line)
		}
	}
	return strings.Join(lines, "\n")
}

func isJSONLine(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch s[0] {
	case '{', '[', '"', '}', ']':
		return true
	}
	if (s[0] >= '0' && s[0] <= '9') || s[0] == '-' {
		return true
	}
	if strings.HasPrefix(s, "true") || strings.HasPrefix(s, "false") || strings.HasPrefix(s, "null") {
		return true
	}
	return false
}

func highlightJSONLine(line string) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787"))  // green
	strStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A5D6FF"))  // cyan
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF"))  // blue
	litStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72"))  // red
	puncStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C9D1D9")) // gray

	var out strings.Builder
	i := 0
	for i < len(line) {
		ch := line[i]

		if ch == ' ' || ch == '\t' {
			out.WriteByte(ch)
			i++
			continue
		}

		if ch == '"' {
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' && j+1 < len(line) {
					j += 2
				} else if line[j] == '"' {
					j++
					break
				} else {
					j++
				}
			}
			token := line[i:j]
			k := j
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k < len(line) && line[k] == ':' {
				out.WriteString(keyStyle.Render(token))
			} else {
				out.WriteString(strStyle.Render(token))
			}
			i = j
			continue
		}

		if (ch >= '0' && ch <= '9') || ch == '-' {
			j := i
			if ch == '-' {
				j++
			}
			for j < len(line) && (unicode.IsDigit(rune(line[j])) || line[j] == '.' || line[j] == 'e' || line[j] == 'E' || line[j] == '+' || line[j] == '-') {
				j++
			}
			out.WriteString(numStyle.Render(line[i:j]))
			i = j
			continue
		}

		if strings.HasPrefix(line[i:], "true") {
			out.WriteString(litStyle.Render("true"))
			i += 4
			continue
		}
		if strings.HasPrefix(line[i:], "false") {
			out.WriteString(litStyle.Render("false"))
			i += 5
			continue
		}
		if strings.HasPrefix(line[i:], "null") {
			out.WriteString(litStyle.Render("null"))
			i += 4
			continue
		}

		out.WriteString(puncStyle.Render(string(ch)))
		i++
	}

	return out.String()
}

// hardWrap first does normal word-wrap (break on spaces), then forces a
// character-level break on any line that's still too wide — this covers
// unbroken tokens like NATS subjects or long URLs that have no spaces
// for lipgloss's word-wrap to break on. ANSI-aware, so JSON syntax
// highlighting in the body survives wrapping intact.
func hardWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	wordWrapped := lipgloss.NewStyle().Width(width).Render(s)
	return wrap.String(wordWrapped, width)
}
