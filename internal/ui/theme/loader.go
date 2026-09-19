package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// expandHome
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// Load reads the palette from an external YAML file (for the designer) and constructs a Theme.
// If the file is not found, it returns Default() without an error; this is not critical
// for application startup.
//
// Example theme file:
//
//	background: "#1e1e2e"
//	foreground: "#cdd6f4"
//	accent: "#89b4fa"
//	...
func Load(path string) (Theme, error) {
	if path == "" {
		return Default(), nil
	}
	path = expandHome(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("read theme %s: %w", path, err)
	}

	p := DefaultPalette()
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Default(), fmt.Errorf("parse theme %s: %w", path, err)
	}

	return New(p), nil
}
