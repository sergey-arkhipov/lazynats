// The streamsview package implements the "Streams" tab: a list of streams on the left,
// and the subjects of the selected stream on the right. Pressing Enter on a subject
// replaces the right panel with a view of the latest messages (contentview); Esc goes back.
package streamsview

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nats-io/nats.go/jetstream"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/contentview"
	"lazynats/internal/ui/keys"
	"lazynats/internal/ui/theme"
)

const (
	defaultTimeout   = 5 * time.Second
	messagesLimit    = 50
	fetchMessagesTTL = 10 * time.Second
)

// Focus — which of the two panels is active (relevant in list mode).
type Focus int

// FocusStreams FocusSubjects - enum
const (
	FocusStreams Focus = iota
	FocusSubjects
)

// mode — right panel - subjects or messages (content)
type mode int

const (
	modeList mode = iota
	modeContent
)

// Dynamic layout
const (
	minListWidth = 20 // minimum for list readability
	maxListWidth = 50 // maximum to prevent the right panel from collapsing to zero
	listRatio    = 3  // denominator: left = width / listRatio, but clamped
)

type layoutKind int

const (
	layoutHorizontal layoutKind = iota
	layoutVertical              // steak
)

// --- list.Item  ---------------------------------------------

type streamItem struct {
	summary natsclient.StreamSummary
}

func (i streamItem) Title() string { return i.summary.Name }
func (i streamItem) Description() string {
	return fmt.Sprintf("%d subj · %d msgs", len(i.summary.Subjects), i.summary.Messages)
}
func (i streamItem) FilterValue() string { return i.summary.Name }

type subjectItem struct {
	subject string
}

func (i subjectItem) Title() string       { return i.subject }
func (i subjectItem) Description() string { return "" }
func (i subjectItem) FilterValue() string { return i.subject }

// --- messages ----------------------------------------------------------

type streamsLoadedMsg struct {
	items []natsclient.StreamSummary
	err   error
}

type messagesLoadedMsg struct {
	stream  string
	subject string
	items   []natsclient.Message
	err     error
}

type streamInfoLoadedMsg struct {
	name string
	info *jetstream.StreamInfo
	err  error
}

type streamConsumersLoadedMsg struct {
	name         string
	consumerInfo []natsclient.ConsumerInfo
	err          error
}

// SelectedStreamMsg move up when stream selected.
type SelectedStreamMsg struct {
	Name string
}

// subjectDelegate renders each subject on up to 2 lines instead of
// truncating to one, so long NATS subjects (which share long common
// prefixes and differ only in the tail) stay readable in the list.
// Styling mirrors the theming applied to the default delegate in New,
// so selection colors/border stay consistent across both lists.
type subjectDelegate struct {
	styles list.DefaultItemStyles
}

func newSubjectDelegate(th theme.Theme) subjectDelegate {
	styles := list.NewDefaultItemStyles()
	styles.SelectedTitle = styles.SelectedTitle.
		Foreground(lipgloss.Color(th.Palette.SelectionFg)).
		BorderForeground(lipgloss.Color(th.Palette.BorderFocus))
	// there's no SelectedDesc here since subjects have no description
	// line, but NormalTitle/SelectedTitle cover both wrapped lines since
	// we render them as a single styled block
	return subjectDelegate{styles: styles}
}

func (d subjectDelegate) Height() int                         { return 2 }
func (d subjectDelegate) Spacing() int                        { return 1 }
func (d subjectDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d subjectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(subjectItem)
	if !ok {
		return
	}

	width := m.Width() - 4
	if width < 1 {
		width = 1
	}

	lines := wrapTwoLines(si.subject, width)

	style := d.styles.NormalTitle
	if index == m.Index() {
		style = d.styles.SelectedTitle
	}
	_, _ = fmt.Fprint(w, style.Render(lines))
}

// --- model ---------------------------------------------------------------

// Model — tab Streams state.
type Model struct {
	client *natsclient.Client
	theme  theme.Theme
	keys   keys.ListKeyMap

	streams  list.Model
	subjects list.Model
	content  contentview.Model

	focus Focus
	mode  mode

	width, height int
	layout        layoutKind

	loadingMessages bool
	err             error
}

// New  Streams model tab
func New(client *natsclient.Client, th theme.Theme) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(th.Palette.SelectionFg)).
		BorderForeground(lipgloss.Color(th.Palette.BorderFocus))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color(th.Palette.Muted)).
		BorderForeground(lipgloss.Color(th.Palette.BorderFocus))

	streams := list.New(nil, delegate, 0, 0)
	streams.Title = "Streams"
	streams.SetShowStatusBar(false)
	streams.SetFilteringEnabled(true)
	streams.Styles.Title = th.Title

	subjects := list.New(nil, newSubjectDelegate(th), 0, 0)
	subjects.Title = "Subjects"
	subjects.SetShowStatusBar(false)
	subjects.SetFilteringEnabled(true)
	subjects.Styles.Title = th.Title

	return Model{
		client:   client,
		theme:    th,
		keys:     keys.DefaultList(),
		streams:  streams,
		subjects: subjects,
		content:  contentview.New(th),
		focus:    FocusStreams,
		mode:     modeList,
	}
}

// Init run initial streams list.
func (m Model) Init() tea.Cmd {
	return m.loadStreamsCmd()
}

func (m Model) loadStreamsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		items, err := client.ListStreams(ctx)
		return streamsLoadedMsg{items: items, err: err}
	}
}

func (m Model) loadMessagesCmd(stream, subject string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchMessagesTTL)
		defer cancel()

		items, err := client.FetchLastMessages(ctx, stream, subject, messagesLimit)
		return messagesLoadedMsg{stream: stream, subject: subject, items: items, err: err}
	}
}

func (m Model) loadStreamInfoCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		info, err := client.StreamInfo(ctx, name)
		return streamInfoLoadedMsg{name: name, info: info, err: err}
	}
}

func (m Model) loadStreamConsumersCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		consumers, err := client.ConsumerList(ctx, name)
		return streamConsumersLoadedMsg{name: name, consumerInfo: consumers, err: err}
	}
}

// SetSize  set layout window size.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height

	// border + padding
	frameW := m.theme.PanelBorder.GetHorizontalFrameSize() // usual 4
	frameH := m.theme.PanelBorder.GetVerticalFrameSize()   // usual 2 (up/down border)

	if width < minListWidth*2+frameW*2 {
		m.layout = layoutVertical
		half := height / 2
		innerW := width - frameW
		innerH := half - frameH

		m.streams.SetSize(innerW, innerH)
		m.subjects.SetSize(innerW, innerH)
		m.content.SetSize(innerW, innerH)
		return
	}

	m.layout = layoutHorizontal
	left := width / listRatio
	if left < minListWidth+frameW {
		left = minListWidth + frameW
	}
	if left > maxListWidth+frameW {
		left = maxListWidth + frameW
	}
	right := width - left

	m.streams.SetSize(left-frameW, height-frameH)
	m.subjects.SetSize(right-frameW, height-frameH)
	m.content.SetSize(right-frameW, height-frameH)
}

// Focused returns the current focus (for the status bar/top hints).
func (m Model) Focused() Focus { return m.focus }

// Filtering indicates whether the filter is currently being edited (used
// at the top level to avoid intercepting global hotkeys like Tab
// while typing).
func (m Model) Filtering() bool {
	return m.streams.FilterState() == list.Filtering || m.subjects.FilterState() == list.Filtering
}

// HasActiveFilter indicates whether a filter exists in any state—either
// being typed or already applied. Used to ensure Esc isn't swallowed as a no-op
// in list mode, but instead reaches list.Model to reset the filter.
func (m Model) HasActiveFilter() bool {
	return m.streams.FilterState() != list.Unfiltered ||
		m.subjects.FilterState() != list.Unfiltered
}

// InContentMode is true if the right panel is currently showing the message
// view rather than the subjects list (needed for contextual hints).
func (m Model) InContentMode() bool { return m.mode == modeContent }

// SelectedStreamName returns the name of the stream currently selected in the list
// (if the list is not empty) — used for the create/delete modals at the top.
func (m Model) SelectedStreamName() (string, bool) {
	sel, ok := m.streams.SelectedItem().(streamItem)
	if !ok {
		return "", false
	}
	return sel.summary.Name, true
}

// Update model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		items := make([]list.Item, len(msg.items))
		for i, s := range msg.items {
			items[i] = streamItem{summary: s}
		}
		m.streams.SetItems(items)
		return m, m.refreshSubjectsForSelection()

	case messagesLoadedMsg:
		m.loadingMessages = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.content.SetContent(msg.subject, subtitleForMessages(msg.stream, len(msg.items)), formatMessages(msg.items))
		return m, nil

	case streamInfoLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.content.SetContent(msg.name, "Stream Info · esc/h/← back · y copy", formatStreamInfo(msg.info))
		return m, nil

	case streamConsumersLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.content.SetContent(msg.name, "Stream Consumers · esc/h/← back · y copy", formatConsumers(msg.consumerInfo, m.theme))
		return m, nil

	case tea.KeyMsg:
		if m.focus == FocusStreams && m.streams.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.streams, cmd = m.streams.Update(msg)
			return m, cmd
		}
		if m.focus == FocusSubjects && m.subjects.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.subjects, cmd = m.subjects.Update(msg)
			return m, cmd
		}
		if m.mode == modeContent {
			if msg.String() == "h" || msg.String() == "left" || msg.String() == "esc" {
				m.mode = modeList
				return m, nil
			}
			var cmd tea.Cmd
			m.content, cmd = m.content.Update(msg)
			return m, cmd
		}

		switch {
		case bkey.Matches(msg, m.keys.Select) || msg.String() == "l" || msg.String() == "right":
			if m.focus == FocusStreams {
				m.focus = FocusSubjects
				return m, nil
			}
			if m.focus == FocusSubjects {
				return m.enterContentMode()
			}
		case msg.String() == "h" || msg.String() == "left":
			if m.focus == FocusSubjects {
				m.focus = FocusStreams
				return m, nil
			}
		case bkey.Matches(msg, m.keys.Info):
			if m.focus == FocusStreams {
				return m.enterStreamInfoMode()
			}
		case bkey.Matches(msg, m.keys.Consumers):
			if m.focus == FocusStreams {
				return m.enterStreamConsumersMode()
			}
		}
	}

	if m.mode == modeContent {
		var cmd tea.Cmd
		m.content, cmd = m.content.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	switch m.focus {
	case FocusStreams:
		prevSelected, _ := m.streams.SelectedItem().(streamItem)
		m.streams, cmd = m.streams.Update(msg)
		newSelected, ok := m.streams.SelectedItem().(streamItem)
		if !ok || prevSelected.summary.Name != newSelected.summary.Name {
			return m, tea.Batch(cmd, m.refreshSubjectsForSelection())
		}
	case FocusSubjects:
		m.subjects, cmd = m.subjects.Update(msg)
	}

	return m, cmd
}

// enterContentMode initiates the loading of messages for the selected subject
// and switches the right panel to view mode.
func (m Model) enterContentMode() (Model, tea.Cmd) {
	streamSel, ok := m.streams.SelectedItem().(streamItem)
	if !ok {
		return m, nil
	}
	subjSel, ok := m.subjects.SelectedItem().(subjectItem)
	if !ok {
		return m, nil
	}

	m.mode = modeContent
	m.loadingMessages = true
	m.content.SetContent(subjSel.subject, "loading...", "")

	return m, m.loadMessagesCmd(streamSel.summary.Name, subjSel.subject)
}

// enterStreamInfoMode initiates the loading of StreamInfo for the selected stream
// and switches the right panel to view mode.
func (m Model) enterStreamInfoMode() (Model, tea.Cmd) {
	streamSel, ok := m.streams.SelectedItem().(streamItem)
	if !ok {
		return m, nil
	}

	m.mode = modeContent
	m.content.SetContent(streamSel.summary.Name, "loading...", "")

	return m, m.loadStreamInfoCmd(streamSel.summary.Name)
}

// enterStreamInfoMode initiates the loading of StreamInfo for the selected stream
// and switches the right panel to view mode.
func (m Model) enterStreamConsumersMode() (Model, tea.Cmd) {
	streamSel, ok := m.streams.SelectedItem().(streamItem)
	if !ok {
		return m, nil
	}

	m.mode = modeContent
	m.content.SetContent(streamSel.summary.Name, "loading...", "")

	return m, m.loadStreamConsumersCmd(streamSel.summary.Name)
}

// refreshSubjectsForSelection updates the right-hand subjects panel
// based on the currently selected stream (without a network request — the subjects are already in StreamSummary).
func (m *Model) refreshSubjectsForSelection() tea.Cmd {
	m.mode = modeList

	sel, ok := m.streams.SelectedItem().(streamItem)
	if !ok {
		m.subjects.SetItems(nil)
		return nil
	}

	items := make([]list.Item, len(sel.summary.Subjects))
	for i, s := range sel.summary.Subjects {
		items[i] = subjectItem{subject: s}
	}
	m.subjects.SetItems(items)

	return func() tea.Msg { return SelectedStreamMsg{Name: sel.summary.Name} }
}

// View renders panels side-by-side (or a list plus a message view).
func (m Model) View() string {
	th := m.theme

	if m.err != nil {
		return th.Danger.Render(fmt.Sprintf("Error: %v", m.err))
	}

	leftStyle := th.PanelBorder
	rightStyle := th.PanelBorder
	if m.focus == FocusStreams && m.mode == modeList {
		leftStyle = th.PanelBorderOn
	} else {
		rightStyle = th.PanelBorderOn
	}

	left := leftStyle.Render(m.streams.View())

	var right string
	if m.mode == modeContent {
		right = rightStyle.Render(m.content.View())
	} else {
		right = rightStyle.Render(m.subjects.View())
	}

	if m.layout == layoutVertical {
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func subtitleForMessages(stream string, count int) string {
	return fmt.Sprintf("stream=%s · %d msg · esc/h/← back · y copy", stream, count)
}

func formatMessages(msgs []natsclient.Message) string {
	if len(msgs) == 0 {
		return "(no msg for subject)"
	}

	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "#%d  %s  %s  (%d bytes)\n",
			m.Sequence, m.Timestamp.Format(time.RFC3339), m.Subject, len(m.Data))
		b.WriteString(strings.Repeat("-", 40))
		b.WriteString("\n")
		// Format JSON
		data := contentview.PrettifyJSON(m.Data)
		b.Write(data)
	}
	return b.String()
}

func formatStreamInfo(info *jetstream.StreamInfo) string {
	if info == nil {
		return "(no data)"
	}

	cfg, st := info.Config, info.State

	var b strings.Builder
	fmt.Fprintf(&b, "Name:         %s\n", cfg.Name)
	fmt.Fprintf(&b, "Subjects:     %s\n", strings.Join(cfg.Subjects, ", "))
	fmt.Fprintf(&b, "Storage:      %s\n", cfg.Storage)
	fmt.Fprintf(&b, "Retention:    %s\n", cfg.Retention)
	fmt.Fprintf(&b, "Replicas:     %d\n", cfg.Replicas)
	fmt.Fprintf(&b, "Max Age:      %s\n", cfg.MaxAge)
	fmt.Fprintf(&b, "Max Msgs:     %d\n", cfg.MaxMsgs)
	fmt.Fprintf(&b, "Max Bytes:    %d\n", cfg.MaxBytes)
	b.WriteString("\n")
	fmt.Fprintf(&b, "Messages:     %d\n", st.Msgs)
	fmt.Fprintf(&b, "Bytes:        %d\n", st.Bytes)
	fmt.Fprintf(&b, "First Seq:    %d  (%s)\n", st.FirstSeq, st.FirstTime.Format(time.RFC3339))
	fmt.Fprintf(&b, "Last Seq:     %d  (%s)\n", st.LastSeq, st.LastTime.Format(time.RFC3339))
	fmt.Fprintf(&b, "Consumers:    %d\n", st.Consumers)

	return b.String()
}

func formatConsumers(items []natsclient.ConsumerInfo, th theme.Theme) string {
	if len(items) == 0 {
		return th.Muted.Render("No consumers found.")
	}

	lines := make([]string, 0, len(items)*2)
	for _, c := range items {
		nameStyle := th.EntryName
		if c.Delivered == 0 {
			nameStyle = th.EntryNameMuted
		}

		lines = append(lines, nameStyle.Render(fmt.Sprintf("• %s", c.Name)))

		lastStr := "-"
		if c.Last != nil {
			lastStr = c.Last.Format("2006-01-02 15:04:05")
		}
		filters := strings.Join(c.FilterSubjects, ", ")
		if filters == "" {
			filters = "-"
		}

		meta := fmt.Sprintf("filters: %s  pending: %d  delivered: %d  last: %s",
			filters, c.NumPending, c.Delivered, lastStr)
		lines = append(lines, th.EntryMeta.Render(meta))
	}

	return strings.Join(lines, "\n")
}

// wrapTwoLines hard-wraps s to at most 2 lines of the given width,
// truncating with an ellipsis on the second line if it still doesn't fit.
// Subjects have no spaces, so this is a straight character-level wrap
// rather than word-wrap.
func wrapTwoLines(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	first := string(runes[:width])
	rest := runes[width:]
	if len(rest) <= width {
		return first + "\n" + string(rest)
	}
	const ellipsis = "…"
	tail := width - lipgloss.Width(ellipsis)
	if tail < 0 {
		tail = 0
	}
	return first + "\n" + string(rest[:tail]) + ellipsis
}
