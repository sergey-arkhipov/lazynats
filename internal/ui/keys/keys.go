// Package keys defines global and local keybindings.
// It has been separated out so that, just like the theme,
// it can be made configurable later (currently, it consists simply of constants + bubbles/key).
package keys

import "github.com/charmbracelet/bubbles/key"

// GlobalKeyMap — key combinations that work in any application context.
type GlobalKeyMap struct {
	Quit       key.Binding
	NextTab    key.Binding
	PrevTab    key.Binding
	FocusLeft  key.Binding
	FocusRight key.Binding
	Refresh    key.Binding
	Help       key.Binding
	New        key.Binding
	Delete     key.Binding
}

// DefaultGlobal — default key set.
func DefaultGlobal() GlobalKeyMap {
	return GlobalKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "exit"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		FocusLeft: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "left panel"),
		),
		FocusRight: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "right panel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "create"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
	}
}

// ListKeyMap — navigation within a keys (streams/buckets/subjects/keys).
type ListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Info   key.Binding
}

// DefaultList — default keys navigation.
func DefaultList() ListKeyMap {
	return ListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Info: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "info"),
		),
	}
}
