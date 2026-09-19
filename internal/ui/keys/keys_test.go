package keys

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultGlobal(t *testing.T) {
	km := DefaultGlobal()

	tests := []struct {
		name     string
		got      key.Binding
		wantKeys []string
		wantHelp key.Help
	}{
		{
			name:     "Quit",
			got:      km.Quit,
			wantKeys: []string{"q", "ctrl+c"},
			wantHelp: key.Help{Key: "q", Desc: "exit"},
		},
		{
			name:     "NextTab",
			got:      km.NextTab,
			wantKeys: []string{"tab"},
			wantHelp: key.Help{Key: "tab", Desc: "next tab"},
		},
		{
			name:     "PrevTab",
			got:      km.PrevTab,
			wantKeys: []string{"shift+tab"},
			wantHelp: key.Help{Key: "shift+tab", Desc: "prev tab"},
		},
		{
			name:     "FocusLeft",
			got:      km.FocusLeft,
			wantKeys: []string{"h", "left"},
			wantHelp: key.Help{Key: "h/←", Desc: "left panel"},
		},
		{
			name:     "FocusRight",
			got:      km.FocusRight,
			wantKeys: []string{"l", "right"},
			wantHelp: key.Help{Key: "l/→", Desc: "right panel"},
		},
		{
			name:     "Refresh",
			got:      km.Refresh,
			wantKeys: []string{"r", "ctrl+r"},
			wantHelp: key.Help{Key: "r", Desc: "refresh"},
		},
		{
			name:     "Help",
			got:      km.Help,
			wantKeys: []string{"?"},
			wantHelp: key.Help{Key: "?", Desc: "help"},
		},
		{
			name:     "New",
			got:      km.New,
			wantKeys: []string{"n"},
			wantHelp: key.Help{Key: "n", Desc: "create"},
		},
		{
			name:     "Delete",
			got:      km.Delete,
			wantKeys: []string{"d"},
			wantHelp: key.Help{Key: "d", Desc: "delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got.Keys(), tt.wantKeys) {
				t.Errorf("Keys() = %v, want %v", tt.got.Keys(), tt.wantKeys)
			}
			if !reflect.DeepEqual(tt.got.Help(), tt.wantHelp) {
				t.Errorf("Help() = %+v, want %+v", tt.got.Help(), tt.wantHelp)
			}
		})
	}
}

func TestDefaultList(t *testing.T) {
	km := DefaultList()

	tests := []struct {
		name     string
		got      key.Binding
		wantKeys []string
		wantHelp key.Help
	}{
		{
			name:     "Up",
			got:      km.Up,
			wantKeys: []string{"k", "up"},
			wantHelp: key.Help{Key: "k/↑", Desc: "up"},
		},
		{
			name:     "Down",
			got:      km.Down,
			wantKeys: []string{"j", "down"},
			wantHelp: key.Help{Key: "j/↓", Desc: "down"},
		},
		{
			name:     "Select",
			got:      km.Select,
			wantKeys: []string{"enter"},
			wantHelp: key.Help{Key: "enter", Desc: "select"},
		},
		{
			name:     "Info",
			got:      km.Info,
			wantKeys: []string{"i"},
			wantHelp: key.Help{Key: "i", Desc: "info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got.Keys(), tt.wantKeys) {
				t.Errorf("Keys() = %v, want %v", tt.got.Keys(), tt.wantKeys)
			}
			if !reflect.DeepEqual(tt.got.Help(), tt.wantHelp) {
				t.Errorf("Help() = %+v, want %+v", tt.got.Help(), tt.wantHelp)
			}
		})
	}
}
