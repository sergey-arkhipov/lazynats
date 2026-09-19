package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestExpandHome(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "tilde prefix",
			path: "~/theme.yaml",
			want: filepath.Join(tmpHome, "theme.yaml"),
		},
		{
			name: "tilde with subdir",
			path: "~/.config/app/theme.yaml",
			want: filepath.Join(tmpHome, ".config", "app", "theme.yaml"),
		},
		{
			name: "no tilde absolute",
			path: "/absolute/path/theme.yaml",
			want: "/absolute/path/theme.yaml",
		},
		{
			name: "relative path",
			path: "relative/theme.yaml",
			want: "relative/theme.yaml",
		},
		{
			name: "empty",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.path)
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExpandHome_NoHome(t *testing.T) {
	t.Setenv("HOME", "") // empty HOME → UserHomeDir return err

	got := expandHome("~/theme.yaml")
	if got != "~/theme.yaml" {
		t.Errorf("expandHome(%q) = %q, want %q", "~/theme.yaml", got, "~/theme.yaml")
	}
}

func TestLoad(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		th, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !themesEqual(th, Default()) {
			t.Error("expected Default() theme for empty path")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		th, err := Load("/nonexistent/path/theme.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !themesEqual(th, Default()) {
			t.Error("expected Default() theme when file not found")
		}
	})

	t.Run("valid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "theme.yaml")
		content := `
background: "#000000"
foreground: "#111111"
border: "#222222"
border_focus: "#333333"
accent: "#444444"
muted: "#555555"
danger: "#666666"
success: "#777777"
warning: "#888888"
selection_bg: "#999999"
selection_fg: "#aaaaaa"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		th, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := Palette{
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

		if !reflect.DeepEqual(th.Palette, want) {
			t.Errorf("Palette = %+v, want %+v", th.Palette, want)
		}
	})

	t.Run("partial yaml", func(t *testing.T) {
		// Redefine accent only — get others from DefaultPalette
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "theme.yaml")
		content := `accent: "#deadbe"`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		th, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if th.Palette.Accent != "#deadbe" {
			t.Errorf("Accent = %q, want %q", th.Palette.Accent, "#deadbe")
		}

		def := DefaultPalette()
		if th.Palette.Background != def.Background {
			t.Errorf("Background = %q, want %q", th.Palette.Background, def.Background)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "theme.yaml")
		if err := os.WriteFile(path, []byte("not: valid: yaml: ["), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		th, err := Load(path)
		if err == nil {
			t.Fatal("expected error for invalid yaml")
		}
		if !themesEqual(th, Default()) {
			t.Error("expected Default() theme on parse error")
		}
	})

	t.Run("tilde path", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		path := filepath.Join(tmpHome, "theme.yaml")
		content := `accent: "#deadbe"`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		th, err := Load("~/theme.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := th.Title.GetForeground()
		want := lipgloss.Color("#deadbe")
		if got != want {
			t.Errorf("Title foreground = %v, want %v", got, want)
		}
	})
}

// themesEqual compares two Themes by rendering all styles.
// If this is also used in theme_test.go, move it to a separate helper file.
func themesEqual(a, b Theme) bool {
	if !reflect.DeepEqual(a.Palette, b.Palette) {
		return false
	}
	styles := []struct {
		name string
		a, b lipgloss.Style
	}{
		{"Base", a.Base, b.Base},
		{"PanelBorder", a.PanelBorder, b.PanelBorder},
		{"PanelBorderOn", a.PanelBorderOn, b.PanelBorderOn},
		{"Title", a.Title, b.Title},
		{"ListItem", a.ListItem, b.ListItem},
		{"ListItemSel", a.ListItemSel, b.ListItemSel},
		{"StatusBar", a.StatusBar, b.StatusBar},
		{"StatusBarKey", a.StatusBarKey, b.StatusBarKey},
		{"Muted", a.Muted, b.Muted},
		{"Danger", a.Danger, b.Danger},
		{"Success", a.Success, b.Success},
		{"Warning", a.Warning, b.Warning},
	}
	for _, s := range styles {
		if s.a.Render("x") != s.b.Render("x") {
			return false
		}
	}
	return true
}
