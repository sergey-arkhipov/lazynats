package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "nats://127.0.0.1:4222", cfg.NatsURL)
	assert.Empty(t, cfg.ThemePath)
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join("lazynats", "config.yaml"))
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no tilde", "/absolute/path", "/absolute/path"},
		{"with tilde", "~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"just tilde", "~", home},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, Default().NatsURL, cfg.NatsURL)
	assert.Empty(t, cfg.ThemePath)
}

func TestLoad_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
nats_url: "nats://custom:4222"
theme_path: "/custom/theme.yaml"
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "nats://custom:4222", cfg.NatsURL)
	assert.Equal(t, "/custom/theme.yaml", cfg.ThemePath)
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("LAZYNATS_NATS_URL", "nats://env:4222")
	t.Setenv("LAZYNATS_THEME_PATH", "/env/theme.yaml")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "nats://env:4222", cfg.NatsURL)
	assert.Equal(t, "/env/theme.yaml", cfg.ThemePath)
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`nats_url: "nats://file:4222"`), 0o644))

	t.Setenv("LAZYNATS_NATS_URL", "nats://env:4222")

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "nats://env:4222", cfg.NatsURL) // env wins over file
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, Default().NatsURL, cfg.NatsURL)
}

func TestLoad_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("not: valid: yaml: ["), 0o644))

	_, err := Load(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestLoad_EmptyNatsURLFallsBackToDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	// Explicitly set nats_url to empty string in file
	require.NoError(t, os.WriteFile(configPath, []byte("nats_url: \"\"\n"), 0o644))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, Default().NatsURL, cfg.NatsURL)
}
