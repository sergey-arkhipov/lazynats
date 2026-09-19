package streamsview

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nats-io/nats.go/jetstream"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/theme"
)

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func newTestModel() Model {
	th := theme.Default()
	m := New(nil, th)
	m.SetSize(90, 30)
	return m
}

// ---------------------------------------------------------------------
// Item types
// ---------------------------------------------------------------------

func TestStreamItem(t *testing.T) {
	si := streamItem{summary: natsclient.StreamSummary{
		Name:     "orders",
		Subjects: []string{"orders.>", "orders.new"},
		Messages: 42,
	}}
	if got := si.Title(); got != "orders" {
		t.Errorf("Title() = %q, want %q", got, "orders")
	}
	wantDesc := "2 subj · 42 msgs"
	if got := si.Description(); got != wantDesc {
		t.Errorf("Description() = %q, want %q", got, wantDesc)
	}
	if got := si.FilterValue(); got != "orders" {
		t.Errorf("FilterValue() = %q, want %q", got, "orders")
	}
}

func TestSubjectItem(t *testing.T) {
	si := subjectItem{subject: "orders.created"}
	if got := si.Title(); got != "orders.created" {
		t.Errorf("Title() = %q, want %q", got, "orders.created")
	}
	if got := si.Description(); got != "" {
		t.Errorf("Description() = %q, want %q", got, "")
	}
	if got := si.FilterValue(); got != "orders.created" {
		t.Errorf("FilterValue() = %q, want %q", got, "orders.created")
	}
}

// ---------------------------------------------------------------------
// Model / New / Init / SetSize / getters
// ---------------------------------------------------------------------

func TestNew(t *testing.T) {
	m := newTestModel()

	if m.focus != FocusStreams {
		t.Errorf("focus = %v, want FocusStreams", m.focus)
	}
	if m.mode != modeList {
		t.Errorf("mode = %v, want modeList", m.mode)
	}
	if m.streams.Title != "Streams" {
		t.Errorf("streams.Title = %q, want %q", m.streams.Title, "Streams")
	}
	if m.subjects.Title != "Subjects" {
		t.Errorf("subjects.Title = %q, want %q", m.subjects.Title, "Subjects")
	}
	if m.width != 90 || m.height != 30 {
		t.Errorf("size = (%d,%d), want (90,30)", m.width, m.height)
	}
}

func TestInit(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
}

func TestSetSize(t *testing.T) {
	m := New(nil, theme.Default())
	m.SetSize(120, 40)

	if m.width != 120 || m.height != 40 {
		t.Errorf("size = (%d,%d), want (120,40)", m.width, m.height)
	}
	if m.layout != layoutHorizontal {
		t.Errorf("layout = %v, want layoutHorizontal", m.layout)
	}
}

func TestFocused(t *testing.T) {
	m := newTestModel()
	if m.Focused() != FocusStreams {
		t.Errorf("Focused() = %v, want FocusStreams", m.Focused())
	}
}

func TestInContentMode(t *testing.T) {
	m := newTestModel()
	if m.InContentMode() {
		t.Error("InContentMode() = true, want false")
	}
	m.mode = modeContent
	if !m.InContentMode() {
		t.Error("InContentMode() = false, want true")
	}
}

func TestSelectedStreamName(t *testing.T) {
	m := newTestModel()
	if _, ok := m.SelectedStreamName(); ok {
		t.Error("SelectedStreamName() ok = true on empty list")
	}

	m, _ = m.Update(streamsLoadedMsg{
		items: []natsclient.StreamSummary{{Name: "events"}},
	})

	name, ok := m.SelectedStreamName()
	if !ok || name != "events" {
		t.Errorf("SelectedStreamName() = (%q, %v), want (%q, true)", name, ok, "events")
	}
}

func TestFiltering(t *testing.T) {
	m := newTestModel()
	if m.Filtering() {
		t.Error("Filtering() = true, want false by default")
	}
}

// ---------------------------------------------------------------------
// Update — messages
// ---------------------------------------------------------------------

func TestUpdateStreamsLoaded(t *testing.T) {
	m := newTestModel()

	msg := streamsLoadedMsg{
		items: []natsclient.StreamSummary{
			{Name: "stream-a"},
			{Name: "stream-b"},
		},
	}
	newM, cmd := m.Update(msg)

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if len(newM.streams.Items()) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(newM.streams.Items()))
	}
	if cmd == nil {
		t.Error("expected cmd (refreshSubjectsForSelection) after streamsLoaded")
	}
}

func TestUpdateStreamsLoadedError(t *testing.T) {
	m := newTestModel()
	newM, _ := m.Update(streamsLoadedMsg{err: errors.New("conn refused")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

func TestUpdateMessagesLoaded(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent
	m.loadingMessages = true

	msg := messagesLoadedMsg{
		stream:  "s1",
		subject: "subj",
		items: []natsclient.Message{
			{Sequence: 1, Timestamp: time.Now(), Subject: "subj", Data: []byte(`{"a":1}`)},
		},
	}
	newM, _ := m.Update(msg)

	if newM.loadingMessages {
		t.Error("loadingMessages should be false")
	}
	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
}

func TestUpdateMessagesLoadedError(t *testing.T) {
	m := newTestModel()
	m.loadingMessages = true
	newM, _ := m.Update(messagesLoadedMsg{err: errors.New("fail")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
	if newM.loadingMessages {
		t.Error("loadingMessages should be false on error")
	}
}

func TestUpdateStreamInfoLoaded(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent

	info := &jetstream.StreamInfo{
		Config: jetstream.StreamConfig{Name: "s1"},
		State:  jetstream.StreamState{Msgs: 5},
	}
	newM, _ := m.Update(streamInfoLoadedMsg{name: "s1", info: info})

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
}

func TestUpdateStreamInfoLoadedError(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent
	newM, _ := m.Update(streamInfoLoadedMsg{err: errors.New("fail")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

// ---------------------------------------------------------------------
// Update — keyboard navigation
// ---------------------------------------------------------------------

func TestUpdateKeyMsgRightFocusesSubjects(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1"}}})

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if newM.Focused() != FocusSubjects {
		t.Errorf("focus = %v, want FocusSubjects", newM.Focused())
	}
}

func TestUpdateKeyMsgRightEntersContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1", Subjects: []string{"subj"}}}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus subjects

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if !newM.loadingMessages {
		t.Error("expected loadingMessages true")
	}
	if cmd == nil {
		t.Error("expected cmd (loadMessages)")
	}
}

func TestUpdateKeyMsgEnterEntersContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1", Subjects: []string{"subj"}}}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadMessages)")
	}
}

func TestUpdateKeyMsgArrowRightFocusesSubjects(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1"}}})

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if newM.Focused() != FocusSubjects {
		t.Errorf("focus = %v, want FocusSubjects", newM.Focused())
	}
}

func TestUpdateKeyMsgLeftFocusesStreams(t *testing.T) {
	m := newTestModel()
	m.focus = FocusSubjects

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if newM.Focused() != FocusStreams {
		t.Errorf("focus = %v, want FocusStreams", newM.Focused())
	}
}

func TestUpdateKeyMsgHFocusesStreams(t *testing.T) {
	m := newTestModel()
	m.focus = FocusSubjects

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if newM.Focused() != FocusStreams {
		t.Errorf("focus = %v, want FocusStreams", newM.Focused())
	}
}

func TestUpdateKeyMsgEscLeavesContent(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if newM.InContentMode() {
		t.Error("expected list mode after esc")
	}
}

func TestUpdateKeyMsgInfoEntersStreamInfo(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1"}}})

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadStreamInfo)")
	}
}

// ---------------------------------------------------------------------
// Remaining Update branches
// ---------------------------------------------------------------------

func TestUpdateContentMode_NonKeyMsg(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, cmd := m.Update(msg)

	if newM.mode != modeContent {
		t.Error("expected to stay in content mode")
	}
	_ = cmd
}

func TestUpdateContentMode_KeyMsgNotEsc(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if newM.mode != modeContent {
		t.Error("expected to stay in content mode")
	}
	_ = cmd
}

func TestUpdateStreamsNavigation_ChangesIndex(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{
		items: []natsclient.StreamSummary{
			{Name: "stream-a"},
			{Name: "stream-b"},
		},
	})
	if m.streams.Index() != 0 {
		t.Fatalf("unexpected start index %d", m.streams.Index())
	}

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.streams.Index() == 0 {
		t.Fatal("expected stream index to change after KeyDown")
	}
	if cmd == nil {
		t.Fatal("expected cmd (tea.Batch with refreshSubjectsForSelection) when index changes")
	}
}

func TestUpdateStreamsNavigation_NoIndexChange(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "only-one"}}})
	prevIdx := m.streams.Index()

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if newM.streams.Index() != prevIdx {
		t.Fatalf("expected index to stay %d, got %d", prevIdx, newM.streams.Index())
	}
	_ = cmd
}

func TestUpdateSubjectsNavigation(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1", Subjects: []string{"a", "b"}}}})
	m.focus = FocusSubjects

	prevIdx := m.subjects.Index()
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.subjects.Index() == prevIdx {
		t.Fatal("expected subjects index to change after KeyDown")
	}
	_ = cmd
}

// ---------------------------------------------------------------------
// refreshSubjectsForSelection
// ---------------------------------------------------------------------

func TestRefreshSubjectsForSelection(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "orders", Subjects: []string{"orders.>", "orders.new"}},
	}})

	cmd := m.refreshSubjectsForSelection()
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectedStreamMsg)
	if !ok {
		t.Fatalf("expected SelectedStreamMsg, got %T", msg)
	}
	if sel.Name != "orders" {
		t.Errorf("Name = %q, want %q", sel.Name, "orders")
	}
	if len(m.subjects.Items()) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(m.subjects.Items()))
	}
}

// ---------------------------------------------------------------------
// enterContentMode / enterStreamInfoMode
// ---------------------------------------------------------------------

func TestEnterContentMode(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1", Subjects: []string{"subj"}}}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	newM, cmd := m.enterContentMode()
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if !newM.loadingMessages {
		t.Error("expected loadingMessages true")
	}
	if cmd == nil {
		t.Error("expected cmd (loadMessages)")
	}
}

func TestEnterStreamInfoMode(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{{Name: "s1"}}})

	newM, cmd := m.enterStreamInfoMode()
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadStreamInfo)")
	}
}

// ---------------------------------------------------------------------
// View
// ---------------------------------------------------------------------

func TestViewError(t *testing.T) {
	m := newTestModel()
	m.err = errors.New("network timeout")
	out := m.View()
	if !strings.Contains(out, "Error") {
		t.Errorf("View() missing error label, got: %q", out)
	}
}

func TestViewListMode(t *testing.T) {
	m := newTestModel()
	out := m.View()
	if !strings.Contains(out, "Streams") {
		t.Errorf("View() missing Streams title, got: %q", out)
	}
}

func TestViewContentMode(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent
	m.content.SetContent("mykey", "subtitle", "body text")
	out := m.View()
	if out == "" {
		t.Error("View() returned empty string in content mode")
	}
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func TestSubtitleForMessages(t *testing.T) {
	s := subtitleForMessages("events", 7)
	wantParts := []string{"stream=events", "7 msg", "esc/h/← back"}
	for _, p := range wantParts {
		if !strings.Contains(s, p) {
			t.Errorf("subtitle %q missing %q", s, p)
		}
	}
}

func TestFormatMessages(t *testing.T) {
	msgs := []natsclient.Message{
		{
			Sequence:  1,
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Subject:   "orders.created",
			Data:      []byte(`{"id":42}`),
		},
	}
	s := formatMessages(msgs)
	if !strings.Contains(s, "#1") {
		t.Errorf("missing sequence, got: %s", s)
	}
}

func TestFormatMessagesEmpty(t *testing.T) {
	s := formatMessages(nil)
	if s != "(no msg for subject)" {
		t.Errorf("got %q, want empty message", s)
	}
}

func TestFormatStreamInfoNil(t *testing.T) {
	if got := formatStreamInfo(nil); got != "(no data)" {
		t.Errorf("got %q, want %q", got, "(no data)")
	}
}

func TestFormatStreamInfo(t *testing.T) {
	info := &jetstream.StreamInfo{
		Config: jetstream.StreamConfig{
			Name:      "s1",
			Subjects:  []string{"foo.>"},
			Storage:   jetstream.FileStorage,
			Retention: jetstream.LimitsPolicy,
			Replicas:  1,
			MaxAge:    time.Hour,
			MaxMsgs:   1000,
			MaxBytes:  1024,
		},
		State: jetstream.StreamState{
			Msgs:      10,
			Bytes:     512,
			FirstSeq:  1,
			FirstTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			LastSeq:   10,
			LastTime:  time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Consumers: 2,
		},
	}
	s := formatStreamInfo(info)
	wantParts := []string{"s1", "foo.>", "File", "Limits", "1", "10", "512", "Consumers"}
	for _, p := range wantParts {
		if !strings.Contains(s, p) {
			t.Errorf("status %q missing %q", s, p)
		}
	}
}

// ---------------------------------------------------------------------
// wrapTwoLines
// ---------------------------------------------------------------------

func TestWrapTwoLinesShortStringUnchanged(t *testing.T) {
	got := wrapTwoLines("orders.created", 30)
	if got != "orders.created" {
		t.Errorf("wrapTwoLines() = %q, want unchanged input", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("wrapTwoLines() should not wrap a string shorter than width, got %q", got)
	}
}

func TestWrapTwoLinesExactWidthUnchanged(t *testing.T) {
	s := strings.Repeat("a", 10)
	got := wrapTwoLines(s, 10)
	if got != s {
		t.Errorf("wrapTwoLines() = %q, want unchanged %q", got, s)
	}
}

func TestWrapTwoLinesFitsInTwoLines(t *testing.T) {
	// 15 chars, width 10: first line 10 chars, remaining 5 fit on the
	// second line untouched (no ellipsis needed).
	s := "csquad.csscat.created"[:15]
	got := wrapTwoLines(s, 10)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != s[:10] {
		t.Errorf("first line = %q, want %q", lines[0], s[:10])
	}
	if lines[1] != s[10:] {
		t.Errorf("second line = %q, want %q", lines[1], s[10:])
	}
	if strings.Contains(got, "…") {
		t.Errorf("should not truncate when the remainder fits, got %q", got)
	}
}

func TestWrapTwoLinesTruncatesSecondLine(t *testing.T) {
	// Long NATS-style subject, no spaces — must hard-wrap and truncate
	// the second line with an ellipsis rather than overflow.
	s := "csquad.csscat.20edc095-e896-4721-93f7-2a3c09c1f846.01a0.very.long.tail"
	width := 20
	got := wrapTwoLines(s, width)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != s[:width] {
		t.Errorf("first line = %q, want %q", lines[0], s[:width])
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Errorf("second line = %q, want ellipsis suffix", lines[1])
	}
	if lipglossWidth(lines[1]) > width {
		t.Errorf("second line %q exceeds width %d", lines[1], width)
	}
}

func TestWrapTwoLinesZeroWidth(t *testing.T) {
	// width<1 is clamped to 1 by the caller (Render), but wrapTwoLines
	// itself should not panic on a degenerate width.
	got := wrapTwoLines("orders.created", 0)
	if got == "" {
		t.Error("wrapTwoLines() with width=0 should not return empty for non-empty input")
	}
}

func TestWrapTwoLinesEmptyString(t *testing.T) {
	got := wrapTwoLines("", 10)
	if got != "" {
		t.Errorf("wrapTwoLines(\"\", 10) = %q, want empty", got)
	}
}

// lipglossWidth is a tiny local helper so the test doesn't need to import
// lipgloss just for Width() on plain (non-styled) strings.
func lipglossWidth(s string) int {
	return len([]rune(s))
}

// ---------------------------------------------------------------------
// subjectDelegate
// ---------------------------------------------------------------------

func TestSubjectDelegateHeightAndSpacing(t *testing.T) {
	d := newSubjectDelegate(theme.Default())
	if d.Height() != 2 {
		t.Errorf("Height() = %d, want 2", d.Height())
	}
	if d.Spacing() != 1 {
		t.Errorf("Spacing() = %d, want 1", d.Spacing())
	}
}

func TestSubjectDelegateUpdateIsNoop(t *testing.T) {
	d := newSubjectDelegate(theme.Default())
	if cmd := d.Update(tea.KeyMsg{Type: tea.KeyDown}, nil); cmd != nil {
		t.Error("Update() should be a no-op returning nil cmd")
	}
}

func TestSubjectDelegateRenderWrapsLongSubject(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "s1", Subjects: []string{
			"csquad.csscat.20edc095-e896-4721-93f7-2a3c09c1f846.01a0.tail",
		}},
	}})

	// force the subjects list narrow enough that the subject won't fit
	// on one line, so we can assert Render actually wraps it
	m.subjects.SetSize(20, 10)

	out := m.subjects.View()
	if !strings.Contains(out, "…") {
		t.Errorf("expected wrapped/truncated subject with ellipsis, got:\n%s", out)
	}
}

func TestSubjectDelegateRenderShortSubjectSingleLine(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "s1", Subjects: []string{"orders.new"}},
	}})
	m.subjects.SetSize(90, 10)

	out := m.subjects.View()
	if !strings.Contains(out, "orders.new") {
		t.Errorf("expected full short subject visible, got:\n%s", out)
	}
	if strings.Contains(out, "…") {
		t.Errorf("short subject should not be truncated, got:\n%s", out)
	}
}

func TestSubjectDelegateRenderSelectedVsNormal(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "s1", Subjects: []string{"a.subject", "b.subject"}},
	}})
	m.subjects.SetSize(90, 10)

	// index 0 is selected by default
	firstView := m.subjects.View()

	m.subjects.CursorDown()
	secondView := m.subjects.View()

	if firstView == secondView {
		t.Error("expected selection styling to change the rendered view when cursor moves")
	}
}

func TestSubjectDelegateRenderIgnoresWrongItemType(t *testing.T) {
	d := newSubjectDelegate(theme.Default())
	m := newTestModel()

	var b strings.Builder
	// streamItem is a valid list.Item but not a subjectItem — Render
	// should silently skip it rather than panic.
	d.Render(&b, m.subjects, 0, streamItem{summary: natsclient.StreamSummary{Name: "not-a-subject"}})
	if b.String() != "" {
		t.Errorf("Render() with wrong item type wrote %q, want empty", b.String())
	}
}

// ---------------------------------------------------------------------
// SetSize — subjects delegate width edge cases
// ---------------------------------------------------------------------

func TestSetSizeVeryNarrowDoesNotPanic(t *testing.T) {
	m := New(nil, theme.Default())
	// width small enough that right-4 (padding reserved in Render) would
	// go negative if not clamped
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SetSize/View panicked on narrow width: %v", r)
		}
	}()
	m.SetSize(10, 20)
	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "s1", Subjects: []string{"orders.created.eu-west-1"}},
	}})
	_ = m.View()
}

func TestSetSizeVerticalLayoutSubjectsUsable(t *testing.T) {
	m := New(nil, theme.Default())
	m.SetSize(30, 40) // below minListWidth*2+frameW*2 → vertical layout

	if m.layout != layoutVertical {
		t.Fatalf("expected layoutVertical, got %v", m.layout)
	}

	m, _ = m.Update(streamsLoadedMsg{items: []natsclient.StreamSummary{
		{Name: "s1", Subjects: []string{"orders.created"}},
	}})
	out := m.View()
	if !strings.Contains(out, "orders.created") {
		t.Errorf("expected subject visible in vertical layout, got:\n%s", out)
	}
}
