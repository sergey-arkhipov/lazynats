// Package root is the root tea.Model of the application: Streams/Buckets tabs,
// status bar, create/delete modals and overall layout.
package root

import (
	"context"
	"fmt"
	"strings"
	"time"

	bkey "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/bucketsview"
	"lazynats/internal/ui/keys"
	"lazynats/internal/ui/modal"
	"lazynats/internal/ui/statusbar"
	"lazynats/internal/ui/streamsview"
	"lazynats/internal/ui/theme"
)

const actionTimeout = 10 * time.Second

// contentChrome is the number of rows taken by the tab header and status bar.
const contentChrome = 5

// Tab is the active top-level tab.
type Tab int

// Tab 	- enum
const (
	TabStreams Tab = iota
	TabBuckets
)

// String returns the tab name.
func (t Tab) String() string {
	switch t {
	case TabStreams:
		return "Streams"
	case TabBuckets:
		return "Buckets"
	default:
		return "?"
	}
}

// modalKind is the currently open modal (if any).
type modalKind int

const (
	modalNone modalKind = iota
	modalCreateStream
	modalCreateBucket
	modalDeleteStream
	modalDeleteBucket
)

// actionDoneMsg is the result of a create/delete command to NATS.
type actionDoneMsg struct {
	tab     Tab
	err     error
	success string // banner text on success
}

// Model is the root application model.
type Model struct {
	client *natsclient.Client
	theme  theme.Theme
	global keys.GlobalKeyMap

	tab     Tab
	streams streamsview.Model
	buckets bucketsview.Model
	status  statusbar.Model

	modal        modalKind
	confirm      modal.ConfirmModel
	form         modal.FormModel
	deleteTarget string // stream/bucket name awaiting deletion confirmation

	banner       string
	bannerDanger bool

	width  int
	height int
}

// New builds the root model from an established NATS connection.
func New(client *natsclient.Client, th theme.Theme) Model {
	m := Model{
		client:  client,
		theme:   th,
		global:  keys.DefaultGlobal(),
		tab:     TabStreams,
		streams: streamsview.New(client, th),
		buckets: bucketsview.New(client, th),
		status:  statusbar.New(th),
		modal:   modalNone,
	}
	m.status.SetConnection(true, client.ConnectedURL())
	m.updateHints()
	return m
}

// Init starts loading data for both tabs.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.streams.Init(), m.buckets.Init())
}

// Update is the main message dispatcher.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.status.SetWidth(msg.Width)
		m.confirm.SetSize(msg.Width, msg.Height)
		m.form.SetSize(msg.Width, msg.Height)

		contentHeight := msg.Height - contentChrome
		contentHeight = max(contentHeight, 0)

		m.streams.SetSize(msg.Width, contentHeight)
		m.buckets.SetSize(msg.Width, contentHeight)
		return m, nil

	case modal.FormSubmittedMsg:
		return m.handleFormSubmitted(msg)

	case modal.FormCancelledMsg:
		m.closeModal()
		return m, nil

	case modal.ConfirmResultMsg:
		return m.handleConfirmResult(msg)

	case actionDoneMsg:
		if msg.err != nil {
			m.banner = msg.err.Error()
			m.bannerDanger = true
			return m, nil
		}
		m.banner = msg.success
		m.bannerDanger = false
		switch msg.tab {
		case TabStreams:
			return m, m.streams.Init()
		case TabBuckets:
			return m, m.buckets.Init()
		}
		return m, nil

	case tea.KeyMsg:
		if m.modal != modalNone {
			return m.updateModal(msg)
		}
		if !m.currentFiltering() {
			// esc outside content mode is a no-op (never quits).
			// In content mode esc is passed to tabs where it returns to the list.
			if msg.String() == "esc" && !m.inContentMode() && !m.currentHasActiveFilter() {
				return m, nil
			}
			switch {
			case bkey.Matches(msg, m.global.Quit):
				return m, tea.Quit
			case bkey.Matches(msg, m.global.NextTab):
				m.tab = (m.tab + 1) % 2
				m.banner = ""
				m.updateHints()
				return m, nil
			case bkey.Matches(msg, m.global.PrevTab):
				m.tab = (m.tab + 1) % 2
				m.banner = ""
				m.updateHints()
				return m, nil
			case bkey.Matches(msg, m.global.Refresh):
				// Reset filter
				switch m.tab {
				case TabStreams:
					m.streams.ResetFilter()
				case TabBuckets:
					m.buckets.ResetFilter()
				}
				return m, m.refreshCurrentTab()
			case bkey.Matches(msg, m.global.New):
				return m.openCreateModal()
			case bkey.Matches(msg, m.global.Delete):
				return m.openDeleteModal()
			}
		}
	}

	// While a modal is open all remaining messages (including cursor blink
	// ticks) must reach the modal, not the lists.
	if m.modal != modalNone {
		return m.updateModal(msg)
	}

	var cmd1, cmd2 tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Key events must only reach the active tab — otherwise both
		// streamsview and bucketsview react to the same key (e.g. "/"
		// would start filtering in both, even though only one is visible).
		switch m.tab {
		case TabStreams:
			m.streams, cmd1 = m.streams.Update(keyMsg)
		case TabBuckets:
			m.buckets, cmd2 = m.buckets.Update(keyMsg)
		}
	} else {
		// Non-key messages (async load results, ticks, etc.) are scoped
		// by their own message type/payload, so it's safe — and for
		// initial loads necessary — to forward them to both tabs.
		m.streams, cmd1 = m.streams.Update(msg)
		m.buckets, cmd2 = m.buckets.Update(msg)
	}
	m.updateHints()
	return m, tea.Batch(cmd1, cmd2)
}

// updateModal routes messages to the active modal.
func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.modal {
	case modalCreateStream, modalCreateBucket:
		m.form, cmd = m.form.Update(msg)
	case modalDeleteStream, modalDeleteBucket:
		m.confirm, cmd = m.confirm.Update(msg)
	}
	return m, cmd
}

func (m *Model) closeModal() {
	m.modal = modalNone
	m.deleteTarget = ""
}

// openCreateModal opens a create form depending on the active tab.
func (m Model) openCreateModal() (Model, tea.Cmd) {
	m.banner = ""
	switch m.tab {
	case TabStreams:
		m.modal = modalCreateStream
		m.form = modal.NewForm(m.theme, "New stream",
			"tab — next field · enter — create · esc — cancel",
			[]modal.FieldSpec{
				{Label: "Name", Placeholder: "orders"},
				{Label: "Subjects (comma separated)", Placeholder: "orders.*, orders.created"},
			})
	case TabBuckets:
		m.modal = modalCreateBucket
		m.form = modal.NewForm(m.theme, "New bucket (KV)",
			"enter — create · esc — cancel",
			[]modal.FieldSpec{
				{Label: "Name", Placeholder: "config"},
			})
	default:
		return m, nil
	}
	m.form.SetSize(m.width, m.height)
	return m, nil
}

// openDeleteModal opens a delete confirmation for the currently selected
// stream/bucket on the active tab. Does nothing if nothing is selected.
func (m Model) openDeleteModal() (Model, tea.Cmd) {
	m.banner = ""
	switch m.tab {
	case TabStreams:
		name, ok := m.streams.SelectedStreamName()
		if !ok {
			return m, nil
		}
		m.modal = modalDeleteStream
		m.deleteTarget = name
		m.confirm = modal.NewConfirm(m.theme, "Delete stream",
			fmt.Sprintf("Stream %q will be deleted along with all messages. This action is irreversible.", name), true)
	case TabBuckets:
		name, ok := m.buckets.SelectedBucketName()
		if !ok {
			return m, nil
		}
		m.modal = modalDeleteBucket
		m.deleteTarget = name
		m.confirm = modal.NewConfirm(m.theme, "Delete bucket",
			fmt.Sprintf("Bucket %q will be destroyed along with all keys. This action is irreversible.", name), true)
	default:
		return m, nil
	}
	m.confirm.SetSize(m.width, m.height)
	return m, nil
}

// handleFormSubmitted validates input and sends a create command to NATS.
// On validation error the modal stays open with an error message.
func (m Model) handleFormSubmitted(msg modal.FormSubmittedMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalCreateStream:
		name := msg.Values[0]
		if name == "" {
			m.form.SetError("Stream name cannot be empty")
			return m, nil
		}
		var subjects []string
		for _, s := range strings.Split(msg.Values[1], ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				subjects = append(subjects, s)
			}
		}
		if len(subjects) == 0 {
			m.form.SetError("At least one subject is required")
			return m, nil
		}
		m.closeModal()
		return m, m.createStreamCmd(name, subjects)

	case modalCreateBucket:
		name := msg.Values[0]
		if name == "" {
			m.form.SetError("Bucket name cannot be empty")
			return m, nil
		}
		m.closeModal()
		return m, m.createBucketCmd(name)
	}
	return m, nil
}

// handleConfirmResult executes (or cancels) a pending deletion.
func (m Model) handleConfirmResult(msg modal.ConfirmResultMsg) (tea.Model, tea.Cmd) {
	modalKind := m.modal
	target := m.deleteTarget
	m.closeModal()

	if !msg.Confirmed {
		return m, nil
	}

	switch modalKind {
	case modalDeleteStream:
		return m, m.deleteStreamCmd(target)
	case modalDeleteBucket:
		return m, m.deleteBucketCmd(target)
	}
	return m, nil
}

func (m Model) createStreamCmd(name string, subjects []string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		err := client.CreateStream(ctx, name, subjects)
		return actionDoneMsg{tab: TabStreams, err: err, success: fmt.Sprintf("Stream %q created", name)}
	}
}

func (m Model) deleteStreamCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		err := client.DeleteStream(ctx, name)
		return actionDoneMsg{tab: TabStreams, err: err, success: fmt.Sprintf("Stream %q deleted", name)}
	}
}

func (m Model) createBucketCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		err := client.CreateBucket(ctx, name)
		return actionDoneMsg{tab: TabBuckets, err: err, success: fmt.Sprintf("Bucket %q created", name)}
	}
}

func (m Model) deleteBucketCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		err := client.DeleteBucket(ctx, name)
		return actionDoneMsg{tab: TabBuckets, err: err, success: fmt.Sprintf("Bucket %q deleted", name)}
	}
}

func (m Model) refreshCurrentTab() tea.Cmd {
	switch m.tab {
	case TabStreams:
		return m.streams.Init()
	case TabBuckets:
		return m.buckets.Init()
	default:
		return nil
	}
}

// currentFiltering reports whether the active tab is in filter input mode
// (global hotkeys like q/tab must not be intercepted then).
func (m Model) currentFiltering() bool {
	switch m.tab {
	case TabStreams:
		return m.streams.Filtering()
	case TabBuckets:
		return m.buckets.Filtering()
	default:
		return false
	}
}

func (m Model) currentHasActiveFilter() bool {
	switch m.tab {
	case TabStreams:
		return m.streams.HasActiveFilter()
	case TabBuckets:
		return m.buckets.HasActiveFilter()
	default:
		return false
	}
}

// inContentMode reports whether the active tab is showing content view
// (messages/key value) instead of a list.
func (m Model) inContentMode() bool {
	switch m.tab {
	case TabStreams:
		return m.streams.InContentMode()
	case TabBuckets:
		return m.buckets.InContentMode()
	default:
		return false
	}
}

func (m *Model) updateHints() {
	if m.modal != modalNone {
		m.status.SetHints([]statusbar.Hint{
			{Key: "esc", Action: "cancel"},
		})
		return
	}
	if m.inContentMode() {
		m.status.SetHints([]statusbar.Hint{
			{Key: "esc/h/←", Action: "back to list"},
			{Key: "j/k", Action: "scroll"},
			{Key: "y", Action: "copy all"},
			{Key: "q", Action: "quit"},
		})
		return
	}
	m.status.SetHints([]statusbar.Hint{
		{Key: "tab", Action: "switch tab"},
		{Key: "i", Action: "info"},
		{Key: "c", Action: "consumers"},
		{Key: "n", Action: "create"},
		{Key: "d", Action: "delete"},
		{Key: "/", Action: "filter"},
		{Key: "r", Action: "refresh"},
		{Key: "q", Action: "quit"},
	})
}

// View renders tabs, active tab content, banner, status bar,
// or a modal on top of everything.
func (m Model) View() string {
	th := m.theme

	tabsLine := renderTabs(th, m.tab)

	var content string
	switch m.tab {
	case TabStreams:
		content = m.streams.View()
	case TabBuckets:
		content = m.buckets.View()
	}

	bannerLine := ""
	if m.banner != "" {
		style := th.Success
		if m.bannerDanger {
			style = th.Danger
		}
		bannerLine = style.Padding(0, 1).Render(m.banner)
	}

	base := lipgloss.JoinVertical(lipgloss.Left,
		tabsLine,
		content,
		bannerLine,
		m.status.View(),
	)

	switch m.modal {
	case modalCreateStream, modalCreateBucket:
		return m.form.View()
	case modalDeleteStream, modalDeleteBucket:
		return m.confirm.View()
	default:
		return clampHeight(base, m.height)
	}
}

func renderTabs(th theme.Theme, active Tab) string {
	names := []Tab{TabStreams, TabBuckets}

	var rendered []string
	for _, t := range names {
		style := th.Muted.Padding(0, 2)
		if t == active {
			style = th.Title.Padding(0, 2).Underline(true)
		}
		rendered = append(rendered, style.Render(t.String()))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func clampHeight(s string, height int) string {
	if height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}
