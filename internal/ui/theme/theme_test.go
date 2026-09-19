package theme

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultPalette(t *testing.T) {
	want := Palette{
		Background:  "#1e1e2e",
		Foreground:  "#cdd6f4",
		Border:      "#45475a",
		BorderFocus: "#89b4fa",
		Accent:      "#89b4fa",
		Muted:       "#6c7086",
		Danger:      "#f38ba8",
		Success:     "#a6e3a1",
		Warning:     "#f9e2af",
		SelectionBg: "#313244",
		SelectionFg: "#cdd6f4",
	}

	got := DefaultPalette()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultPalette() = %+v, want %+v", got, want)
	}
}

func TestNew(t *testing.T) {
	// Colors for test
	p := Palette{
		Background:  "#000000",
		Foreground:  "#111111",
		Border:      "#222222",
		BorderFocus: "#333333",
		Accent:      "#444444",
		Muted:       "#555555",
		Danger:      "#666666",
		Success:     "#777777",
		Warning:     "#888888",
		SelectionBg: "#999999",
		SelectionFg: "#aaaaaa",
	}

	th := New(p)

	if !reflect.DeepEqual(th.Palette, p) {
		t.Errorf("Palette = %+v, want %+v", th.Palette, p)
	}

	// helper compared lipgloss.Color from TerminalColor
	assertColor := func(t *testing.T, name string, got lipgloss.TerminalColor, want string) {
		t.Helper()
		if got == nil {
			t.Fatalf("%s: expected color, got nil", name)
		}
		c, ok := got.(lipgloss.Color)
		if !ok {
			t.Fatalf("%s: expected lipgloss.Color, got %T", name, got)
		}
		if string(c) != want {
			t.Errorf("%s = %q, want %q", name, string(c), want)
		}
	}

	t.Run("Base", func(t *testing.T) {
		assertColor(t, "Foreground", th.Base.GetForeground(), p.Foreground)
	})

	t.Run("PanelBorder", func(t *testing.T) {
		s := th.PanelBorder
		if !reflect.DeepEqual(s.GetBorderStyle(), lipgloss.RoundedBorder()) {
			t.Error("expected RoundedBorder")
		}
		assertColor(t, "BorderTopFg", s.GetBorderTopForeground(), p.Border)
		assertColor(t, "BorderRightFg", s.GetBorderRightForeground(), p.Border)
		assertColor(t, "BorderBottomFg", s.GetBorderBottomForeground(), p.Border)
		assertColor(t, "BorderLeftFg", s.GetBorderLeftForeground(), p.Border)

		if s.GetPaddingLeft() != 1 || s.GetPaddingRight() != 1 {
			t.Errorf("horizontal padding = (%d, %d), want (1, 1)", s.GetPaddingLeft(), s.GetPaddingRight())
		}
		if s.GetPaddingTop() != 0 || s.GetPaddingBottom() != 0 {
			t.Errorf("vertical padding = (%d, %d), want (0, 0)", s.GetPaddingTop(), s.GetPaddingBottom())
		}
	})

	t.Run("PanelBorderOn", func(t *testing.T) {
		s := th.PanelBorderOn
		if !reflect.DeepEqual(s.GetBorderStyle(), lipgloss.RoundedBorder()) {
			t.Error("expected RoundedBorder")
		}
		assertColor(t, "BorderTopFg", s.GetBorderTopForeground(), p.BorderFocus)
		assertColor(t, "BorderRightFg", s.GetBorderRightForeground(), p.BorderFocus)
		assertColor(t, "BorderBottomFg", s.GetBorderBottomForeground(), p.BorderFocus)
		assertColor(t, "BorderLeftFg", s.GetBorderLeftForeground(), p.BorderFocus)
	})

	t.Run("Title", func(t *testing.T) {
		assertColor(t, "Foreground", th.Title.GetForeground(), p.Accent)
		if !th.Title.GetBold() {
			t.Error("expected Bold")
		}
	})

	t.Run("ListItem", func(t *testing.T) {
		assertColor(t, "Foreground", th.ListItem.GetForeground(), p.Foreground)
	})

	t.Run("ListItemSel", func(t *testing.T) {
		s := th.ListItemSel
		assertColor(t, "Background", s.GetBackground(), p.SelectionBg)
		assertColor(t, "Foreground", s.GetForeground(), p.SelectionFg)
		if !s.GetBold() {
			t.Error("expected Bold")
		}
	})

	t.Run("StatusBar", func(t *testing.T) {
		assertColor(t, "Foreground", th.StatusBar.GetForeground(), p.Muted)
	})

	t.Run("StatusBarKey", func(t *testing.T) {
		s := th.StatusBarKey
		assertColor(t, "Foreground", s.GetForeground(), p.Accent)
		if !s.GetBold() {
			t.Error("expected Bold")
		}
	})

	t.Run("Muted", func(t *testing.T) {
		assertColor(t, "Foreground", th.Muted.GetForeground(), p.Muted)
	})

	t.Run("Danger", func(t *testing.T) {
		assertColor(t, "Foreground", th.Danger.GetForeground(), p.Danger)
	})

	t.Run("Success", func(t *testing.T) {
		assertColor(t, "Foreground", th.Success.GetForeground(), p.Success)
	})

	t.Run("Warning", func(t *testing.T) {
		assertColor(t, "Foreground", th.Warning.GetForeground(), p.Warning)
	})
}

func TestDefault(t *testing.T) {
	got := Default()
	want := New(DefaultPalette())

	// Compare palette
	if !reflect.DeepEqual(got.Palette, want.Palette) {
		t.Error("Palette mismatch")
	}

	// Compare style by render — lipgloss.Style has unexported fields
	assertSameRender := func(t *testing.T, name string, a, b lipgloss.Style) {
		t.Helper()
		if a.Render("x") != b.Render("x") {
			t.Errorf("%s render mismatch", name)
		}
	}

	assertSameRender(t, "Base", got.Base, want.Base)
	assertSameRender(t, "PanelBorder", got.PanelBorder, want.PanelBorder)
	assertSameRender(t, "PanelBorderOn", got.PanelBorderOn, want.PanelBorderOn)
	assertSameRender(t, "Title", got.Title, want.Title)
	assertSameRender(t, "ListItem", got.ListItem, want.ListItem)
	assertSameRender(t, "ListItemSel", got.ListItemSel, want.ListItemSel)
	assertSameRender(t, "StatusBar", got.StatusBar, want.StatusBar)
	assertSameRender(t, "StatusBarKey", got.StatusBarKey, want.StatusBarKey)
	assertSameRender(t, "Muted", got.Muted, want.Muted)
	assertSameRender(t, "Danger", got.Danger, want.Danger)
	assertSameRender(t, "Success", got.Success, want.Success)
	assertSameRender(t, "Warning", got.Warning, want.Warning)
}
