package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantVersion bool
		wantErr     bool
		checkPath   bool
		wantPath    string
	}{
		{
			name:      "defaults",
			args:      []string{},
			checkPath: true,
		},
		{
			name:     "custom config",
			args:     []string{"--config", "/tmp/lazynats.yaml"},
			wantPath: "/tmp/lazynats.yaml",
		},
		{
			name:        "long version flag",
			args:        []string{"--version"},
			wantVersion: true,
			checkPath:   true,
		},
		{
			name:        "short version flag",
			args:        []string{"-v"},
			wantVersion: true,
			checkPath:   true,
		},
		{
			name:    "unknown flag",
			args:    []string{"-badflag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, showVersion, err := parseFlags(tt.args)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantVersion, showVersion)

			if tt.checkPath {
				assert.NotEmpty(t, opts.ConfigPath)
			} else if tt.wantPath != "" {
				assert.Equal(t, tt.wantPath, opts.ConfigPath)
			}
		})
	}
}

func TestRun_Version(t *testing.T) {
	code := run([]string{"--version"})
	assert.Equal(t, 0, code)
}

func TestRun_VersionShort(t *testing.T) {
	code := run([]string{"-v"})
	assert.Equal(t, 0, code)
}

func TestRun_ParseError(t *testing.T) {
	code := run([]string{"-badflag"})
	assert.Equal(t, 1, code)
}
