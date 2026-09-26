// Package bucketsview implements the "Buckets" (KV) tab: a list of buckets
// on the left, keys of the selected bucket on the right. Enter on a key
// swaps the right panel for a value viewer (contentview); Esc goes back.
package bucketsview

import (
	"context"
	"fmt"
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

const defaultTimeout = 5 * time.Second

// Focus — which of the two panels is active (relevant in list mode).
type Focus int

// FocusBuckets - enum
const (
	FocusBuckets Focus = iota
	FocusKeys
)

// mode — right panel mode: key list or value viewer.
type mode int

const (
	modeList mode = iota
	modeContent
)

// layoutKind — horizontal side-by-side or vertical stack.
type layoutKind int

const (
	layoutHorizontal layoutKind = iota
	layoutVertical
)

const (
	minListWidth = 20
	maxListWidth = 50
	listRatio    = 3
)

// --- list.Item implementations ---------------------------------------------

type bucketItem struct {
	summary natsclient.BucketSummary
}

func (i bucketItem) Title() string { return i.summary.Name }
func (i bucketItem) Description() string {
	if i.summary.Keys == 0 {
		return "Empty KV bucket"
	}
	return fmt.Sprintf("%d keys · %s", i.summary.Keys, humanizeBytes(i.summary.Bytes))
}
func (i bucketItem) FilterValue() string { return i.summary.Name }

type keyItem struct {
	key string
}

func (i keyItem) Title() string       { return i.key }
func (i keyItem) Description() string { return "" }
func (i keyItem) FilterValue() string { return i.key }

// --- messages ------------------------------------------------------------

type bucketsLoadedMsg struct {
	items []natsclient.BucketSummary
	err   error
}

type keysLoadedMsg struct {
	bucket string
	items  []string
	err    error
}

type valueLoadedMsg struct {
	bucket string
	key    string
	entry  entrySnapshot
	err    error
}

type bucketStatusLoadedMsg struct {
	bucket string
	status jetstream.KeyValueStatus
	err    error
}

// entrySnapshot — the small subset of jetstream.KeyValueEntry the UI needs
// (we don't carry the interface itself out through a tea.Msg, for simplicity).
type entrySnapshot struct {
	value    []byte
	revision uint64
	created  time.Time
	op       string
}

// SelectedKeyMsg is sent upward when a specific key is selected.
type SelectedKeyMsg struct {
	Bucket string
	Key    string
}

// --- model -----------------------------------------------------------

// Model — state of the Buckets tab.
type Model struct {
	client *natsclient.Client
	theme  theme.Theme
	keys   keys.ListKeyMap

	buckets list.Model
	keysL   list.Model
	content contentview.Model

	focus Focus
	mode  mode

	width, height int
	layout        layoutKind

	err error
}

// New creates the Buckets tab model.
func New(client *natsclient.Client, th theme.Theme) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(th.Palette.SelectionFg)).
		BorderForeground(lipgloss.Color(th.Palette.BorderFocus))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color(th.Palette.Muted)).
		BorderForeground(lipgloss.Color(th.Palette.BorderFocus))

	buckets := list.New(nil, delegate, 0, 0)
	buckets.Title = "Buckets"
	buckets.SetShowStatusBar(false)
	buckets.SetFilteringEnabled(true)
	buckets.Styles.Title = th.Title

	keysL := list.New(nil, delegate, 0, 0)
	keysL.Title = "Keys"
	keysL.SetShowStatusBar(false)
	keysL.SetFilteringEnabled(true)
	keysL.Styles.Title = th.Title

	return Model{
		client:  client,
		theme:   th,
		keys:    keys.DefaultList(),
		buckets: buckets,
		keysL:   keysL,
		content: contentview.New(th),
		focus:   FocusBuckets,
		mode:    modeList,
	}
}

// Init kicks off the initial bucket list load.
func (m Model) Init() tea.Cmd {
	return m.loadBucketsCmd()
}

// ResetFilter clears any active filter on the buckets list.
func (m *Model) ResetFilter() {
	m.buckets.ResetFilter()
	m.keysL.ResetFilter()
}

func (m Model) loadBucketsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		items, err := client.ListBuckets(ctx)
		return bucketsLoadedMsg{items: items, err: err}
	}
}

func (m Model) loadKeysCmd(bucket string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		items, err := client.BucketKeys(ctx, bucket)
		return keysLoadedMsg{bucket: bucket, items: items, err: err}
	}
}

func (m Model) loadValueCmd(bucket, key string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		entry, err := client.KeyValue(ctx, bucket, key)
		if err != nil {
			return valueLoadedMsg{bucket: bucket, key: key, err: err}
		}
		return valueLoadedMsg{bucket: bucket, key: key, entry: entrySnapshot{
			value:    entry.Value(),
			revision: entry.Revision(),
			created:  entry.Created(),
			op:       entry.Operation().String(),
		}}
	}
}

func (m Model) loadBucketStatusCmd(bucket string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		status, err := client.BucketStatus(ctx, bucket)
		return bucketStatusLoadedMsg{bucket: bucket, status: status, err: err}
	}
}

// SetSize fits both columns into the available area (roughly 1/3 - 2/3).
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height

	// How many characters the border + padding eat up.
	frameW := m.theme.PanelBorder.GetHorizontalFrameSize() // usually 4
	frameH := m.theme.PanelBorder.GetVerticalFrameSize()   // usually 2 (top/bottom border)

	if width < minListWidth*2+frameW*2 {
		m.layout = layoutVertical
		half := height / 2
		innerW := width - frameW
		innerH := half - frameH

		m.buckets.SetSize(innerW, innerH)
		m.keysL.SetSize(innerW, innerH)
		m.content.SetSize(innerW, innerH)
		return
	}

	m.layout = layoutHorizontal
	left := width / listRatio
	left = max(left, minListWidth+frameW)
	left = min(left, maxListWidth+frameW)
	right := width - left

	m.buckets.SetSize(left-frameW, height-frameH)
	m.keysL.SetSize(right-frameW, height-frameH)
	m.content.SetSize(right-frameW, height-frameH)
}

// Focused returns the currently active panel.
func (m Model) Focused() Focus { return m.focus }

// Filtering reports whether a filter is currently being *edited* (used
// upstream to avoid intercepting global hotkeys like Tab while typing).
// This intentionally does NOT include FilterApplied — see HasActiveFilter
// for that case. Conflating the two previously caused every key except
// arrow navigation to stop working once a filter was applied, because
// the upstream global-hotkey switch was gated on "not filtering".
func (m Model) Filtering() bool {
	return m.buckets.FilterState() == list.Filtering || m.keysL.FilterState() == list.Filtering
}

// HasActiveFilter reports whether a filter is active in any state — being
// edited or already applied. Used so Esc isn't swallowed as a list-mode
// no-op upstream and instead reaches list.Model, where it either cancels
// active editing or clears an applied filter.
func (m Model) HasActiveFilter() bool {
	return m.buckets.FilterState() != list.Unfiltered ||
		m.keysL.FilterState() != list.Unfiltered
}

// InContentMode reports whether the right panel is currently showing a
// key's value rather than the key list.
func (m Model) InContentMode() bool { return m.mode == modeContent }

// SelectedBucketName returns the name of the currently selected bucket in
// the list (if the list isn't empty) — used for the create/delete modals
// upstream.
func (m Model) SelectedBucketName() (string, bool) {
	sel, ok := m.buckets.SelectedItem().(bucketItem)
	if !ok {
		return "", false
	}
	return sel.summary.Name, true
}

// Update handles bubbletea messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bucketsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		items := make([]list.Item, len(msg.items))
		for i, b := range msg.items {
			items[i] = bucketItem{summary: b}
		}
		m.buckets.SetItems(items)
		return m, m.loadKeysForSelection()

	case keysLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Guard against a stale response: if the user filtered/moved to a
		// different bucket while this request was in flight, an older
		// request for a previously-selected bucket could complete after
		// a newer one and clobber the key list with the wrong content.
		sel, ok := m.buckets.SelectedItem().(bucketItem)
		if !ok || sel.summary.Name != msg.bucket {
			return m, nil
		}
		m.err = nil
		items := make([]list.Item, len(msg.items))
		for i, k := range msg.items {
			items[i] = keyItem{key: k}
		}
		m.keysL.SetItems(items)
		return m, nil

	case valueLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.content.SetBytes(msg.key, subtitleForValue(msg.bucket, msg.entry), msg.entry.value)
		return m, nil

	case bucketStatusLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.content.SetContent(msg.bucket, "Bucket Info · esc/h/← back · y copy", formatBucketStatus(msg.status))
		return m, nil

	case tea.KeyMsg:
		// While the filter input is actively being edited, forward the key
		// straight to the list — don't intercept it up here.
		if m.focus == FocusBuckets && m.buckets.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.buckets, cmd = m.buckets.Update(msg)
			return m, cmd
		}
		if m.focus == FocusKeys && m.keysL.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.keysL, cmd = m.keysL.Update(msg)
			return m, cmd
		}

		// Esc on an *applied* (not actively-edited) filter must also reach
		// the list so it can clear it — otherwise it falls through to the
		// list-mode branches below, which have no case for "esc" and it's
		// effectively swallowed, leaving the filter stuck on.
		if msg.String() == "esc" && m.mode == modeList {
			if m.focus == FocusBuckets && m.buckets.FilterState() == list.FilterApplied {
				var cmd tea.Cmd
				m.buckets, cmd = m.buckets.Update(msg)
				return m, cmd
			}
			if m.focus == FocusKeys && m.keysL.FilterState() == list.FilterApplied {
				var cmd tea.Cmd
				m.keysL, cmd = m.keysL.Update(msg)
				return m, cmd
			}
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
			if m.focus == FocusBuckets {
				m.focus = FocusKeys
				return m, nil
			}
			if m.focus == FocusKeys {
				return m.enterContentMode()
			}
		case msg.String() == "h" || msg.String() == "left":
			if m.focus == FocusKeys {
				m.focus = FocusBuckets
				return m, nil
			}
		case bkey.Matches(msg, m.keys.Info):
			if m.focus == FocusBuckets {
				return m.enterBucketInfoMode()
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
	case FocusBuckets:
		// Compare by item identity, not by numeric Index(): when a filter
		// is applied (e.g. right after pressing Enter while filtering),
		// the underlying item set changes but the cursor's numeric
		// position can stay the same, silently pointing at a different
		// bucket. Index()-based comparison misses that; comparing the
		// actual selected item catches it.
		prevSelected, _ := m.buckets.SelectedItem().(bucketItem)
		m.buckets, cmd = m.buckets.Update(msg)
		newSelected, ok := m.buckets.SelectedItem().(bucketItem)
		if !ok || prevSelected.summary.Name != newSelected.summary.Name {
			return m, tea.Batch(cmd, m.loadKeysForSelection())
		}
	case FocusKeys:
		m.keysL, cmd = m.keysL.Update(msg)
	}

	return m, cmd
}

// enterContentMode starts loading the value for the selected key and
// switches the right panel into viewer mode.
func (m Model) enterContentMode() (Model, tea.Cmd) {
	bucketSel, ok := m.buckets.SelectedItem().(bucketItem)
	if !ok {
		return m, nil
	}
	keySel, ok := m.keysL.SelectedItem().(keyItem)
	if !ok {
		return m, nil
	}

	m.mode = modeContent
	m.content.SetContent(keySel.key, "loading...", "")

	return m, m.loadValueCmd(bucketSel.summary.Name, keySel.key)
}

// enterBucketInfoMode starts loading the status of the selected bucket and
// switches the right panel into viewer mode.
func (m Model) enterBucketInfoMode() (Model, tea.Cmd) {
	bucketSel, ok := m.buckets.SelectedItem().(bucketItem)
	if !ok {
		return m, nil
	}

	m.mode = modeContent
	m.content.SetContent(bucketSel.summary.Name, "loading...", "")

	return m, m.loadBucketStatusCmd(bucketSel.summary.Name)
}

// loadKeysForSelection starts loading keys for the currently selected bucket.
func (m *Model) loadKeysForSelection() tea.Cmd {
	m.mode = modeList

	sel, ok := m.buckets.SelectedItem().(bucketItem)
	if !ok {
		m.keysL.SetItems(nil)
		return nil
	}
	return m.loadKeysCmd(sel.summary.Name)
}

// View renders the panels side-by-side (or list + value viewer).
func (m Model) View() string {
	th := m.theme

	if m.err != nil {
		return th.Danger.Render(fmt.Sprintf("Error: %v", m.err))
	}

	leftStyle := th.PanelBorder
	rightStyle := th.PanelBorder
	if m.focus == FocusBuckets && m.mode == modeList {
		leftStyle = th.PanelBorderOn
	} else {
		rightStyle = th.PanelBorderOn
	}

	left := leftStyle.Render(m.buckets.View())

	var right string
	if m.mode == modeContent {
		right = rightStyle.Render(m.content.View())
	} else {
		right = rightStyle.Render(m.keysL.View())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func subtitleForValue(bucket string, e entrySnapshot) string {
	return fmt.Sprintf("bucket=%s · rev=%d · %s · %s · esc/h/← back · y copy",
		bucket, e.revision, e.op, e.created.Format(time.RFC3339))
}

func formatBucketStatus(s jetstream.KeyValueStatus) string {
	if s == nil {
		return "(no data)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Bucket:        %s\n", s.Bucket())
	fmt.Fprintf(&b, "Values:        %d\n", s.Values())
	fmt.Fprintf(&b, "Bytes:         %d\n", s.Bytes())
	fmt.Fprintf(&b, "History:       %d\n", s.History())
	fmt.Fprintf(&b, "TTL:           %s\n", s.TTL())
	fmt.Fprintf(&b, "Backing Store: %s\n", s.BackingStore())
	fmt.Fprintf(&b, "Compressed:    %t\n", s.IsCompressed())

	return b.String()
}

// humanizeBytes formats a byte count into a human-readable size.
func humanizeBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
