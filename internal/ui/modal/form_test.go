package modal

import (
	"os"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"lazynats/internal/ui/theme"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// TestMain forces a color profile so lipgloss actually emits ANSI codes
// under `go test` (no TTY) instead of silently rendering plain text.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// ---------------------------------------------------------------------
// NewForm
// ---------------------------------------------------------------------

func TestNewForm_BuildsFieldsFromSpecs(t *testing.T) {
	specs := []FieldSpec{
		{Label: "Name", Placeholder: "orders"},
		{Label: "Subjects (comma's split)", Placeholder: "orders.*, orders.created"},
	}
	m := NewForm(theme.Theme{}, "New stream", "tab — next field", specs)

	if m.title != "New stream" {
		t.Errorf("title = %q, want %q", m.title, "New stream")
	}
	if m.hint != "tab — next field" {
		t.Errorf("hint = %q, want %q", m.hint, "tab — next field")
	}
	if len(m.fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(m.fields))
	}
	for i, spec := range specs {
		if m.fields[i].label != spec.Label {
			t.Errorf("fields[%d].label = %q, want %q", i, m.fields[i].label, spec.Label)
		}
		if m.fields[i].input.Placeholder != spec.Placeholder {
			t.Errorf("fields[%d].input.Placeholder = %q, want %q", i, m.fields[i].input.Placeholder, spec.Placeholder)
		}
		if m.fields[i].input.CharLimit != 512 {
			t.Errorf("fields[%d].input.CharLimit = %d, want 512", i, m.fields[i].input.CharLimit)
		}
		if m.fields[i].input.Width != 46 {
			t.Errorf("fields[%d].input.Width = %d, want 46", i, m.fields[i].input.Width)
		}
	}
}

func TestNewForm_OnlyFirstFieldStartsFocused(t *testing.T) {
	specs := []FieldSpec{{Label: "A"}, {Label: "B"}, {Label: "C"}}
	m := NewForm(theme.Theme{}, "t", "h", specs)

	if m.focus != 0 {
		t.Errorf("focus = %d, want 0", m.focus)
	}
	if !m.fields[0].input.Focused() {
		t.Error("expected fields[0] to start focused")
	}
	for i := 1; i < len(m.fields); i++ {
		if m.fields[i].input.Focused() {
			t.Errorf("expected fields[%d] to start unfocused", i)
		}
	}
}

func TestNewForm_EmptySpecsProducesNoFields(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", nil)
	if len(m.fields) != 0 {
		t.Errorf("len(fields) = %d, want 0", len(m.fields))
	}
}

// ---------------------------------------------------------------------
// SetSize / SetError / Values
// ---------------------------------------------------------------------

func TestSetSize(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}})
	m.SetSize(100, 40)
	if m.width != 100 || m.height != 40 {
		t.Errorf("size = (%d,%d), want (100,40)", m.width, m.height)
	}
}

func TestSetError(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}})
	if m.errMsg != "" {
		t.Fatalf("expected empty errMsg initially, got %q", m.errMsg)
	}
	m.SetError("Name cannot be empty")
	if m.errMsg != "Name cannot be empty" {
		t.Errorf("errMsg = %q, want %q", m.errMsg, "Name cannot be empty")
	}
}

func TestValues_TrimsWhitespaceAndPreservesOrder(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})
	m.fields[0].input.SetValue("  orders  ")
	m.fields[1].input.SetValue("orders.*, orders.created")

	got := m.Values()
	want := []string{"orders", "orders.*, orders.created"}
	if len(got) != len(want) {
		t.Fatalf("Values() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Values()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValues_EmptyFieldsGiveEmptyStrings(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}})
	got := m.Values()
	if len(got) != 1 || got[0] != "" {
		t.Errorf("Values() = %v, want [\"\"]", got)
	}
}

// ---------------------------------------------------------------------
// Update: esc
// ---------------------------------------------------------------------

func TestUpdate_EscReturnsFormCancelledMsg(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command for esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(FormCancelledMsg); !ok {
		t.Errorf("expected FormCancelledMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------
// Update: focus cycling
// ---------------------------------------------------------------------

func TestUpdate_TabAdvancesFocusAndWraps(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}, {Label: "C"}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != 1 {
		t.Fatalf("after 1 tab, focus = %d, want 1", m.focus)
	}
	if m.fields[0].input.Focused() || !m.fields[1].input.Focused() {
		t.Error("expected focus to move from field 0 to field 1")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != 2 {
		t.Fatalf("after 2 tabs, focus = %d, want 2", m.focus)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != 0 {
		t.Fatalf("after 3 tabs (wrap), focus = %d, want 0", m.focus)
	}
	if !m.fields[0].input.Focused() {
		t.Error("expected focus to wrap back to field 0")
	}
	if m.fields[1].input.Focused() || m.fields[2].input.Focused() {
		t.Error("expected only field 0 to be focused after wrap")
	}
}

func TestUpdate_DownActsLikeTab(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focus != 1 {
		t.Errorf("focus after down = %d, want 1", m.focus)
	}
}

func TestUpdate_ShiftTabMovesFocusBackwardAndWraps(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}, {Label: "C"}})

	// From field 0, shift+tab should wrap backward to the last field (2).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != 2 {
		t.Fatalf("focus = %d, want 2 (wrap backward)", m.focus)
	}
	if !m.fields[2].input.Focused() || m.fields[0].input.Focused() {
		t.Error("expected field 2 focused, field 0 blurred after backward wrap")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != 1 {
		t.Fatalf("focus = %d, want 1", m.focus)
	}
}

func TestUpdate_UpActsLikeShiftTab(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.focus != 1 {
		t.Errorf("focus after up (wrap from 0) = %d, want 1", m.focus)
	}
}

// ---------------------------------------------------------------------
// Update: enter
// ---------------------------------------------------------------------

func TestUpdate_EnterOnNonLastField_AdvancesFocusWithoutSubmitting(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.focus != 1 {
		t.Errorf("focus = %d, want 1", m.focus)
	}
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(FormSubmittedMsg); ok {
			t.Error("did not expect FormSubmittedMsg from enter on a non-last field")
		}
	}
}

func TestUpdate_EnterOnLastField_SubmitsWithCurrentValues(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}})
	m.fields[0].input.SetValue("  config  ")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command for enter on last field, got nil")
	}
	msg := cmd()
	submitted, ok := msg.(FormSubmittedMsg)
	if !ok {
		t.Fatalf("expected FormSubmittedMsg, got %T", msg)
	}
	if len(submitted.Values) != 1 || submitted.Values[0] != "config" {
		t.Errorf("submitted.Values = %v, want [\"config\"]", submitted.Values)
	}
}

func TestUpdate_EnterAdvancesThroughAllFieldsThenSubmits(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})
	m.fields[0].input.SetValue("orders")
	m.fields[1].input.SetValue("orders.*")

	// First enter: field 0 -> field 1, no submission yet.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		if _, ok := cmd().(FormSubmittedMsg); ok {
			t.Fatal("did not expect submission after first enter")
		}
	}

	// Second enter: now on the last field -> submits.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submission command on second enter")
	}
	submitted, ok := cmd().(FormSubmittedMsg)
	if !ok {
		t.Fatal("expected FormSubmittedMsg on second enter")
	}
	want := []string{"orders", "orders.*"}
	for i := range want {
		if submitted.Values[i] != want[i] {
			t.Errorf("Values[%d] = %q, want %q", i, submitted.Values[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------
// Update: character input goes to the focused field only
// ---------------------------------------------------------------------

func TestUpdate_CharacterInputUpdatesOnlyFocusedField(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "A"}, {Label: "B"}})

	m, _ = m.Update(runeKey('x'))
	if m.fields[0].input.Value() != "x" {
		t.Errorf("fields[0].input.Value() = %q, want %q", m.fields[0].input.Value(), "x")
	}
	if m.fields[1].input.Value() != "" {
		t.Errorf("fields[1].input.Value() = %q, want empty (not focused)", m.fields[1].input.Value())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(runeKey('y'))
	if m.fields[1].input.Value() != "y" {
		t.Errorf("fields[1].input.Value() = %q, want %q", m.fields[1].input.Value(), "y")
	}
	if m.fields[0].input.Value() != "x" {
		t.Errorf("fields[0].input.Value() changed unexpectedly to %q", m.fields[0].input.Value())
	}
}

// ---------------------------------------------------------------------
// View
// ---------------------------------------------------------------------

func TestView_ContainsTitleLabelsAndHint(t *testing.T) {
	m := NewForm(theme.Theme{}, "New bucket (KV)", "enter — create · esc — cancel", []FieldSpec{
		{Label: "Name", Placeholder: "config"},
	})
	m.SetSize(80, 24)

	out := stripANSI(m.View())
	if !strings.Contains(out, "New bucket (KV)") {
		t.Errorf("View() missing title, got: %q", out)
	}
	if !strings.Contains(out, "Name:") {
		t.Errorf("View() missing field label, got: %q", out)
	}
	if !strings.Contains(out, "enter — create · esc — cancel") {
		t.Errorf("View() missing hint, got: %q", out)
	}
}

func TestView_ShowsErrorMessageWhenSet(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "Name"}})
	m.SetSize(80, 24)
	m.SetError("Name cannot be empty")

	out := stripANSI(m.View())
	if !strings.Contains(out, "Name cannot be empty") {
		t.Errorf("View() missing error message, got: %q", out)
	}
}

func TestView_NoErrorMessageWhenNotSet(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "Name"}})
	m.SetSize(80, 24)

	out := stripANSI(m.View())
	if strings.Contains(out, "cannot be empty") {
		t.Errorf("View() unexpectedly contains an error message: %q", out)
	}
}

func TestView_DoesNotPanicWithoutSetSize(t *testing.T) {
	m := NewForm(theme.Theme{}, "t", "h", []FieldSpec{{Label: "Name"}})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked without SetSize: %v", r)
		}
	}()
	_ = m.View()
}
