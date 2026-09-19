// The theme package centralizes all application colors and styles.
// The idea is for a designer to modify the Palette or an external YAML file
// without touching component code—components request styles from the Theme
// rather than defining lipgloss.Color values inline.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette — a set of named colors. This is the minimum that can
// be moved to an external config for the designer (see loader.go).
type Palette struct {
	Background  string `yaml:"background"`
	Foreground  string `yaml:"foreground"`
	Border      string `yaml:"border"`
	BorderFocus string `yaml:"border_focus"`
	Accent      string `yaml:"accent"`
	Muted       string `yaml:"muted"`
	Danger      string `yaml:"danger"`
	Success     string `yaml:"success"`
	Warning     string `yaml:"warning"`
	SelectionBg string `yaml:"selection_bg"`
	SelectionFg string `yaml:"selection_fg"`
}

// DefaultPalette — default dark theme
func DefaultPalette() Palette {
	return Palette{
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
}

// Theme — ready-to-use lipgloss styles derived from the Palette.
// UI components should reference this rather than creating styles themselves.
type Theme struct {
	Palette Palette

	Base          lipgloss.Style
	PanelBorder   lipgloss.Style
	PanelBorderOn lipgloss.Style // active panel
	Title         lipgloss.Style
	ListItem      lipgloss.Style
	ListItemSel   lipgloss.Style
	StatusBar     lipgloss.Style
	StatusBarKey  lipgloss.Style
	Muted         lipgloss.Style
	Danger        lipgloss.Style
	Success       lipgloss.Style
	Warning       lipgloss.Style
}

// New  Theme from Palette.
func New(p Palette) Theme {
	base := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Foreground))

	panelBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(p.Border)).
		Padding(0, 1)

	panelBorderOn := panelBorder.
		BorderForeground(lipgloss.Color(p.BorderFocus))

	return Theme{
		Palette: p,

		Base:          base,
		PanelBorder:   panelBorder,
		PanelBorderOn: panelBorderOn,

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.Accent)),

		ListItem: base,

		ListItemSel: lipgloss.NewStyle().
			Background(lipgloss.Color(p.SelectionBg)).
			Foreground(lipgloss.Color(p.SelectionFg)).
			Bold(true),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Muted)),

		StatusBarKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Accent)).
			Bold(true),

		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color(p.Muted)),
		Danger:  lipgloss.NewStyle().Foreground(lipgloss.Color(p.Danger)),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success)),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color(p.Warning)),
	}
}

// Default return derault theme
func Default() Theme {
	return New(DefaultPalette())
}
