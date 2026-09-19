package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	badConfig := filepath.Join(tmpDir, "bad.yaml")
	require.NoError(t, os.WriteFile(badConfig, []byte("not: valid: yaml: ["), 0o644))

	_, err := Setup(Options{ConfigPath: badConfig})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config")
}

func TestSetup_InvalidTheme(t *testing.T) {
	// This test assumes theme.Load returns an error for malformed theme files.
	// If your theme loader falls back to defaults for invalid files,
	// adjust the assertion or remove this test.
	tmpDir := t.TempDir()

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	themePath := filepath.Join(tmpDir, "bad-theme.yaml")

	require.NoError(t, os.WriteFile(cfgPath, []byte("nats_url: nats://127.0.0.1:4222\ntheme_path: "+themePath+"\n"), 0o644))
	require.NoError(t, os.WriteFile(themePath, []byte("invalid: yaml: syntax: ["), 0o644))

	_, err := Setup(Options{ConfigPath: cfgPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load theme")
}

func TestSetup_NATSConnectionFailed(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	// Port 1 is extremely unlikely to accept NATS connections.
	require.NoError(t, os.WriteFile(cfgPath, []byte("nats_url: nats://127.0.0.1:1\n"), 0o644))

	_, err := Setup(Options{
		ConfigPath:     cfgPath,
		ConnectTimeout: 200 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to nats")
}
