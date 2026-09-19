package modal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazynats/internal/ui/theme"
)

// ---------------------------------------------------------------------
// NewConfirm / SetSize
// ---------------------------------------------------------------------

func TestNewConfirm_SetsFields(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "Delete stream", `Stream "orders" will be destroyed.`, true)

	if m.title != "Delete stream" {
		t.Errorf("title = %q, want %q", m.title, "Delete stream")
	}
	if m.message != `Stream "orders" will be destroyed.` {
		t.Errorf("message = %q, want %q", m.message, `Stream "orders" will be destroyed.`)
	}
	if !m.danger {
		t.Error("expected danger = true")
	}
}

func TestNewConfirm_NonDangerDefault(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "m", false)
	if m.danger {
		t.Error("expected danger = false")
	}
}

func TestConfirmModel_SetSize(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "m", false)
	m.SetSize(100, 40)
	if m.width != 100 || m.height != 40 {
		t.Errorf("size = (%d,%d), want (100,40)", m.width, m.height)
	}
}

// ---------------------------------------------------------------------
// Update: confirm keys
// ---------------------------------------------------------------------

func confirmResult(t *testing.T, cmd tea.Cmd) ConfirmResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	return result
}

func TestUpdate_ConfirmKeysReturnConfirmedTrue(t *testing.T) {
	for _, key := range []string{"y", "Y", "enter"} {
		t.Run(key, func(t *testing.T) {
			m := NewConfirm(theme.Theme{}, "t", "m", false)
			var keyMsg tea.KeyMsg
			if key == "enter" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			}
			_, cmd := m.Update(keyMsg)
			result := confirmResult(t, cmd)
			if !result.Confirmed {
				t.Errorf("key %q: Confirmed = false, want true", key)
			}
		})
	}
}

func TestUpdate_CancelKeysReturnConfirmedFalse(t *testing.T) {
	for _, key := range []string{"n", "N", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := NewConfirm(theme.Theme{}, "t", "m", false)
			var keyMsg tea.KeyMsg
			if key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			}
			_, cmd := m.Update(keyMsg)
			result := confirmResult(t, cmd)
			if result.Confirmed {
				t.Errorf("key %q: Confirmed = true, want false", key)
			}
		})
	}
}

func TestUpdate_UnhandledKeyReturnsNoCommand(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "m", false)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Errorf("expected nil command for unhandled key, got a command")
	}
}

func TestUpdate_NonKeyMsgReturnsNoCommand(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "m", false)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("expected nil command for non-key message, got a command")
	}
}

func TestUpdate_DoesNotMutateModel(t *testing.T) {
	// Update has a value receiver and returns a (possibly) new model;
	// the caller is expected to use the returned value. This just pins
	// that the returned model round-trips the same fields, i.e.
	// Update doesn't accidentally drop state.
	m := NewConfirm(theme.Theme{}, "t", "m", true)
	m.SetSize(80, 24)
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if got.title != "t" || got.message != "m" || !got.danger || got.width != 80 || got.height != 24 {
		t.Errorf("Update() dropped state: %+v", got)
	}
}

// ---------------------------------------------------------------------
// View
// ---------------------------------------------------------------------

func TestConfirmView_ContainsTitleMessageAndHint(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "Delete bucket", `Bucket "config" will be delete with all keys.`, true)
	m.SetSize(100, 30)

	out := stripANSI(m.View())
	if !strings.Contains(out, "Delete bucket") {
		t.Errorf("View() missing title, got: %q", out)
	}
	if !strings.Contains(out, `Bucket "config" will be delete with all keys.`) {
		t.Errorf("View() missing message, got: %q", out)
	}
	if !strings.Contains(out, "y / enter — confirm") {
		t.Errorf("View() missing confirm hint, got: %q", out)
	}
	if !strings.Contains(out, "n / esc — cancel") {
		t.Errorf("View() missing cancel hint, got: %q", out)
	}
}

func TestConfirmView_DangerChangesStylingNotText(t *testing.T) {
	msg := "This action cannot be undone."
	dangerModel := NewConfirm(theme.Theme{}, "t", msg, true)
	dangerModel.SetSize(80, 24)
	safeModel := NewConfirm(theme.Theme{}, "t", msg, false)
	safeModel.SetSize(80, 24)

	rawDanger := dangerModel.View()
	rawSafe := safeModel.View()

	if stripANSI(rawDanger) != stripANSI(rawSafe) {
		t.Errorf("visible text should be identical regardless of danger flag.\n danger: %q\n safe:   %q",
			stripANSI(rawDanger), stripANSI(rawSafe))
	}
	// With a real theme (distinct Danger vs Base styles) these would
	// differ; with the zero-value theme.Theme{} used in tests both
	// styles are identity renders, so we can't assert inequality here
	// without depending on theme internals. We only assert the safe,
	// theme-independent invariant above (same visible text either way).
}

func TestConfirmView_DoesNotPanicWithoutSetSize(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "m", false)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked without SetSize: %v", r)
		}
	}()
	_ = m.View()
}

func TestConfirmView_EmptyMessageDoesNotPanic(t *testing.T) {
	m := NewConfirm(theme.Theme{}, "t", "", false)
	m.SetSize(80, 24)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked with empty message: %v", r)
		}
	}()
	out := stripANSI(m.View())
	if !strings.Contains(out, "t") {
		t.Errorf("View() missing title even with empty message: %q", out)
	}
}
