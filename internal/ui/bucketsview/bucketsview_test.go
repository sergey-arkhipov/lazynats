package bucketsview

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nats-io/nats.go/jetstream"

	"lazynats/internal/natsclient"
	"lazynats/internal/ui/theme"
)

// ---------------------------------------------------------------------
// Fake KV status
// ---------------------------------------------------------------------

type fakeKVStatus struct {
	bucket       string
	values       uint64
	bytes        uint64
	history      int64
	ttl          time.Duration
	backingStore string
	isCompressed bool
}

func (f *fakeKVStatus) Bucket() string       { return f.bucket }
func (f *fakeKVStatus) Values() uint64       { return f.values }
func (f *fakeKVStatus) Bytes() uint64        { return f.bytes }
func (f *fakeKVStatus) History() int64       { return f.history }
func (f *fakeKVStatus) TTL() time.Duration   { return f.ttl }
func (f *fakeKVStatus) BackingStore() string { return f.backingStore }
func (f *fakeKVStatus) IsCompressed() bool   { return f.isCompressed }

func (f *fakeKVStatus) Config() jetstream.KeyValueConfig {
	return jetstream.KeyValueConfig{Bucket: f.bucket}
}
func (f *fakeKVStatus) LimitMarkerTTL() time.Duration     { return 0 }
func (f *fakeKVStatus) Metadata() map[string]string       { return nil }
func (f *fakeKVStatus) StreamInfo() *jetstream.StreamInfo { return nil }

var _ jetstream.KeyValueStatus = (*fakeKVStatus)(nil)

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

func TestBucketItem_Empty(t *testing.T) {
	bi := bucketItem{summary: natsclient.BucketSummary{Name: "orders"}}
	if got := bi.Title(); got != "orders" {
		t.Errorf("Title() = %q, want %q", got, "orders")
	}
	if got := bi.Description(); got != "Empty KV bucket" {
		t.Errorf("Description() = %q, want %q", got, "Empty KV bucket")
	}
	if got := bi.FilterValue(); got != "orders" {
		t.Errorf("FilterValue() = %q, want %q", got, "orders")
	}
}

func TestBucketItem_WithKeys(t *testing.T) {
	bi := bucketItem{summary: natsclient.BucketSummary{
		Name:   "orders",
		Keys:   42,
		Values: 100,
		Bytes:  2048,
	}}
	if got := bi.Description(); got != "42 keys · 2.0 KB" {
		t.Errorf("Description() = %q, want %q", got, "42 keys · 2.0 KB")
	}
}

func TestKeyItem(t *testing.T) {
	ki := keyItem{key: "user:42"}
	if got := ki.Title(); got != "user:42" {
		t.Errorf("Title() = %q, want %q", got, "user:42")
	}
	if got := ki.Description(); got != "" {
		t.Errorf("Description() = %q, want %q", got, "")
	}
	if got := ki.FilterValue(); got != "user:42" {
		t.Errorf("FilterValue() = %q, want %q", got, "user:42")
	}
}

// ---------------------------------------------------------------------
// Model / New / Init / SetSize / getters
// ---------------------------------------------------------------------

func TestNew(t *testing.T) {
	m := newTestModel()

	if m.focus != FocusBuckets {
		t.Errorf("focus = %v, want FocusBuckets", m.focus)
	}
	if m.mode != modeList {
		t.Errorf("mode = %v, want modeList", m.mode)
	}
	if m.buckets.Title != "Buckets" {
		t.Errorf("buckets.Title = %q, want %q", m.buckets.Title, "Buckets")
	}
	if m.keysL.Title != "Keys" {
		t.Errorf("keysL.Title = %q, want %q", m.keysL.Title, "Keys")
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
	if m.Focused() != FocusBuckets {
		t.Errorf("Focused() = %v, want FocusBuckets", m.Focused())
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

func TestSelectedBucketName(t *testing.T) {
	m := newTestModel()
	if _, ok := m.SelectedBucketName(); ok {
		t.Error("SelectedBucketName() ok = true on empty list")
	}

	m, _ = m.Update(bucketsLoadedMsg{
		items: []natsclient.BucketSummary{{Name: "cfg"}},
	})

	name, ok := m.SelectedBucketName()
	if !ok || name != "cfg" {
		t.Errorf("SelectedBucketName() = (%q, %v), want (%q, true)", name, ok, "cfg")
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

func TestUpdateBucketsLoaded(t *testing.T) {
	m := newTestModel()

	msg := bucketsLoadedMsg{
		items: []natsclient.BucketSummary{
			{Name: "bucket-a"},
			{Name: "bucket-b"},
		},
	}
	newM, cmd := m.Update(msg)

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if len(newM.buckets.Items()) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(newM.buckets.Items()))
	}
	if cmd == nil {
		t.Error("expected cmd (loadKeysForSelection) after bucketsLoaded")
	}
}

func TestUpdateBucketsLoadedError(t *testing.T) {
	m := newTestModel()
	newM, _ := m.Update(bucketsLoadedMsg{err: errors.New("conn refused")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

func TestUpdateKeysLoaded(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})

	msg := keysLoadedMsg{bucket: "b1", items: []string{"k1", "k2"}}
	newM, _ := m.Update(msg)

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if len(newM.keysL.Items()) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(newM.keysL.Items()))
	}
}

func TestUpdateKeysLoadedError(t *testing.T) {
	m := newTestModel()
	newM, _ := m.Update(keysLoadedMsg{err: errors.New("fail")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

func TestUpdateValueLoaded(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m.mode = modeContent // simulate entry

	msg := valueLoadedMsg{
		bucket: "b1",
		key:    "k1",
		entry: entrySnapshot{
			value:    []byte("hello"),
			revision: 7,
			created:  time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			op:       "Put",
		},
	}
	newM, _ := m.Update(msg)

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
}

func TestUpdateValueLoadedError(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent
	newM, _ := m.Update(valueLoadedMsg{err: errors.New("not found")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

func TestUpdateBucketStatusLoaded(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m.mode = modeContent

	fs := &fakeKVStatus{bucket: "b1", values: 5, bytes: 128}
	msg := bucketStatusLoadedMsg{bucket: "b1", status: fs}
	newM, _ := m.Update(msg)

	if newM.err != nil {
		t.Fatalf("unexpected err: %v", newM.err)
	}
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
}

func TestUpdateBucketStatusLoadedError(t *testing.T) {
	m := newTestModel()
	m.mode = modeContent
	newM, _ := m.Update(bucketStatusLoadedMsg{err: errors.New("fail")})
	if newM.err == nil {
		t.Error("expected err to be set")
	}
}

// ---------------------------------------------------------------------
// Update — keyboard navigation
// ---------------------------------------------------------------------

func TestUpdateKeyMsgRightFocusesKeys(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if newM.Focused() != FocusKeys {
		t.Errorf("focus = %v, want FocusKeys", newM.Focused())
	}
}

func TestUpdateKeyMsgRightEntersContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus keys

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadValue)")
	}
}

func TestUpdateKeyMsgEnterEntersContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus keys

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadValue)")
	}
}

func TestUpdateKeyMsgArrowRightFocusesKeys(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if newM.Focused() != FocusKeys {
		t.Errorf("focus = %v, want FocusKeys", newM.Focused())
	}
}

func TestUpdateKeyMsgLeftFocusesBuckets(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus keys

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if newM.Focused() != FocusBuckets {
		t.Errorf("focus = %v, want FocusBuckets", newM.Focused())
	}
}

func TestUpdateKeyMsgHFocusesBuckets(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus keys

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if newM.Focused() != FocusBuckets {
		t.Errorf("focus = %v, want FocusBuckets", newM.Focused())
	}
}

func TestUpdateKeyMsgEscLeavesContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // focus keys
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // enter content

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if newM.InContentMode() {
		t.Error("expected list mode after esc")
	}
}

func TestUpdateKeyMsgHLeavesContent(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // enter content

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if newM.InContentMode() {
		t.Error("expected list mode after h in content")
	}

	m.mode = modeContent
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if newM.InContentMode() {
		t.Error("expected list mode after left in content")
	}
}

func TestUpdateKeyMsgInfoEntersBucketInfo(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if !newM.InContentMode() {
		t.Error("expected content mode")
	}
	if cmd == nil {
		t.Error("expected cmd (loadBucketStatus)")
	}
}

// ---------------------------------------------------------------------
// Update — remaining branches
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

func TestUpdateBucketsNavigation_ChangesIndex(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{
		items: []natsclient.BucketSummary{
			{Name: "bucket-a"},
			{Name: "bucket-b"},
		},
	})
	if m.buckets.Index() != 0 {
		t.Fatalf("unexpected start index %d", m.buckets.Index())
	}

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if newM.buckets.Index() == 0 {
		t.Fatal("expected bucket index to change after KeyDown")
	}
	if cmd == nil {
		t.Fatal("expected cmd when index changes")
	}
}

func TestUpdateBucketsNavigation_NoIndexChange(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{
		items: []natsclient.BucketSummary{{Name: "only-one"}},
	})
	prevIdx := m.buckets.Index()

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if newM.buckets.Index() != prevIdx {
		t.Fatalf("expected bucket index to stay %d, got %d", prevIdx, newM.buckets.Index())
	}
	_ = cmd
}

func TestUpdateKeysNavigation(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1", "k2"}})
	m.focus = FocusKeys

	prevIdx := m.keysL.Index()
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if newM.keysL.Index() == prevIdx {
		t.Fatal("expected keys index to change after KeyDown")
	}
	_ = cmd
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
	if !strings.Contains(out, "Buckets") {
		t.Errorf("View() missing Buckets title, got: %q", out)
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

func TestSubtitleForValue(t *testing.T) {
	e := entrySnapshot{
		value:    []byte("payload"),
		revision: 9,
		created:  time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
		op:       "Put",
	}
	s := subtitleForValue("orders", e)

	wantParts := []string{"bucket=orders", "rev=9", "Put", "2024-03-15T10:30:00Z"}
	for _, part := range wantParts {
		if !strings.Contains(s, part) {
			t.Errorf("subtitle %q missing %q", s, part)
		}
	}
}

func TestFormatBucketStatusNil(t *testing.T) {
	if got := formatBucketStatus(nil); got != "(no data)" {
		t.Errorf("formatBucketStatus(nil) = %q, want %q", got, "(no data)")
	}
}

func TestFormatBucketStatus(t *testing.T) {
	fs := &fakeKVStatus{
		bucket:       "cfg",
		values:       42,
		bytes:        2048,
		history:      3,
		ttl:          time.Hour,
		backingStore: "JetStream",
		isCompressed: true,
	}
	s := formatBucketStatus(fs)

	wantParts := []string{"cfg", "42", "2048", "3", "1h0m0s", "JetStream", "true"}
	for _, part := range wantParts {
		if !strings.Contains(s, part) {
			t.Errorf("status %q missing %q", s, part)
		}
	}
}

// ---------------------------------------------------------------------
// HasActiveFilter
// ---------------------------------------------------------------------

func TestHasActiveFilter_Default(t *testing.T) {
	m := newTestModel()
	if m.HasActiveFilter() {
		t.Error("HasActiveFilter() = true, want false by default")
	}
}

func TestHasActiveFilter_WhileEditing(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}, {Name: "b2"}}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m.buckets.FilterState() != list.Filtering {
		t.Fatalf("expected FilterState Filtering, got %v", m.buckets.FilterState())
	}
	if !m.HasActiveFilter() {
		t.Error("HasActiveFilter() = false while editing filter, want true")
	}
}

func TestHasActiveFilter_AfterApplied(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}, {Name: "b2"}}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', '1'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.buckets.FilterState() != list.FilterApplied {
		t.Fatalf("expected FilterState FilterApplied, got %v", m.buckets.FilterState())
	}
	if !m.HasActiveFilter() {
		t.Error("HasActiveFilter() = false after filter applied, want true")
	}
	// Filtering() must stay false here — this is the distinction the two
	// methods exist for: global hotkeys should work again once the
	// filter is applied, only Esc handling needs HasActiveFilter.
	if m.Filtering() {
		t.Error("Filtering() = true after filter applied, want false")
	}
}

func TestHasActiveFilter_KeysList(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1", "k2"}})
	m.focus = FocusKeys

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.HasActiveFilter() {
		t.Error("HasActiveFilter() = false while editing keys filter, want true")
	}
}

// ---------------------------------------------------------------------
// Esc clearing an applied filter
// ---------------------------------------------------------------------

func TestUpdateEscClearsAppliedFilter_Buckets(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}, {Name: "b2"}}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', '1'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.buckets.FilterState() != list.FilterApplied {
		t.Fatalf("setup: expected FilterApplied, got %v", m.buckets.FilterState())
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if newM.buckets.FilterState() != list.Unfiltered {
		t.Errorf("FilterState after esc = %v, want Unfiltered", newM.buckets.FilterState())
	}
}

func TestUpdateEscClearsAppliedFilter_Keys(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"alpha", "beta"}})
	m.focus = FocusKeys

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'l'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.keysL.FilterState() != list.FilterApplied {
		t.Fatalf("setup: expected FilterApplied, got %v", m.keysL.FilterState())
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if newM.keysL.FilterState() != list.Unfiltered {
		t.Errorf("FilterState after esc = %v, want Unfiltered", newM.keysL.FilterState())
	}
}

func TestUpdateEscInContentMode_DoesNotTouchFilter(t *testing.T) {
	// Esc in content mode should leave content mode, not attempt to
	// clear a filter (mode==modeList guard on the new esc branch).
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "b1"}}})
	m, _ = m.Update(keysLoadedMsg{bucket: "b1", items: []string{"k1"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // enter content

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if newM.InContentMode() {
		t.Error("expected list mode after esc from content mode")
	}
}

// ---------------------------------------------------------------------
// Selection change via filtering — identity comparison, not Index()
// ---------------------------------------------------------------------

func TestUpdateFilterNarrowsSelection_TriggersKeysRefresh(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{
		items: []natsclient.BucketSummary{
			{Name: "alpha"},
			{Name: "beta"},
		},
	})
	sel, ok := m.SelectedBucketName()
	if !ok || sel != "alpha" {
		t.Fatalf("setup: selected = %q, want alpha", sel)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	// Typing "b" alone is enough to narrow the fuzzy match down to just
	// "beta". The resulting cmd is tea.Batch(inputCmd, filterItemsCmd);
	// we need filterItemsCmd's result (list.FilterMatchesMsg) — the
	// message that actually updates m.buckets.filteredItems — fed back
	// in, exactly as the real tea.Program runtime would.
	var typeCmd tea.Cmd
	m, typeCmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if typeCmd == nil {
		t.Fatal("expected a cmd from typing into the filter")
	}

	batch, ok := typeCmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from a filtering keystroke, got %T", typeCmd())
	}

	var sawRefreshCmd bool
	for _, c := range batch {
		if c == nil {
			continue
		}
		innerMsg := c()
		if innerMsg == nil {
			continue
		}
		if _, ok := innerMsg.(list.FilterMatchesMsg); !ok {
			// unrelated message (e.g. a textinput cursor blink) —
			// apply it but don't assert on its cmd
			m, _ = m.Update(innerMsg)
			continue
		}
		var innerCmd tea.Cmd
		m, innerCmd = m.Update(innerMsg)
		if innerCmd != nil {
			sawRefreshCmd = true
		}
	}

	if !sawRefreshCmd {
		t.Fatal("expected a refresh cmd (loadKeysForSelection) once FilterMatchesMsg narrowed the selection to a different bucket")
	}

	sel, ok = m.SelectedBucketName()
	if !ok || sel != "beta" {
		t.Fatalf("selected after filtering = %q, want beta", sel)
	}
}

// ---------------------------------------------------------------------
// keysLoadedMsg — stale response guard
// ---------------------------------------------------------------------

func TestUpdateKeysLoaded_IgnoresStaleBucket(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{
		items: []natsclient.BucketSummary{
			{Name: "alpha"},
			{Name: "beta"},
		},
	})
	// currently selected bucket is "alpha"

	// simulate a slow response for a bucket the user has since moved away
	// from — must not clobber the key list
	newM, _ := m.Update(keysLoadedMsg{bucket: "beta", items: []string{"stale-key"}})

	if len(newM.keysL.Items()) != 0 {
		t.Errorf("expected keys list untouched by stale response, got %d items", len(newM.keysL.Items()))
	}
}

func TestUpdateKeysLoaded_AppliesForCurrentBucket(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(bucketsLoadedMsg{items: []natsclient.BucketSummary{{Name: "alpha"}}})

	newM, _ := m.Update(keysLoadedMsg{bucket: "alpha", items: []string{"k1", "k2"}})

	if len(newM.keysL.Items()) != 2 {
		t.Errorf("expected 2 keys for the currently selected bucket, got %d", len(newM.keysL.Items()))
	}
}

func TestUpdateKeysLoaded_NoBucketSelected(t *testing.T) {
	// empty bucket list — SelectedItem() returns nil/not-ok, so the
	// stale-response guard itself must not panic when there's nothing
	// selected at all.
	m := newTestModel()
	newM, _ := m.Update(keysLoadedMsg{bucket: "anything", items: []string{"k1"}})
	if len(newM.keysL.Items()) != 0 {
		t.Errorf("expected no keys applied with nothing selected, got %d", len(newM.keysL.Items()))
	}
}
