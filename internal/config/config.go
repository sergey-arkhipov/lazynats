// Package config handles the minimal application configuration:
// currently only the NATS server connection string and an optional external theme path.
//
// Sources (in order of increasing priority): defaults -> config file
// -> environment variables with the LAZYNATS_ prefix. This gives
// automatic variables "out of the box" via viper.AutomaticEnv, for example:
//
//	LAZYNATS_NATS_URL=nats://prod:4222 lazynats
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// EnvPrefix is the environment variable prefix that viper picks up
// automatically (LAZYNATS_NATS_URL, LAZYNATS_THEME_PATH, ...).
const EnvPrefix = "LAZYNATS"

// Config is the root configuration structure for lazynats.
type Config struct {
	// NatsURL is the NATS connection string, e.g. "nats://127.0.0.1:4222"
	NatsURL string `mapstructure:"nats_url"`

	// ThemePath is an optional path to an external theme file (for designers).
	// If empty, the built-in default theme is used.
	ThemePath string `mapstructure:"theme_path"`
}

// Default returns the default config (local NATS without TLS/credentials).
func Default() Config {
	return Config{
		NatsURL: "nats://127.0.0.1:4222",
	}
}

// DefaultPath returns the default config path: ~/.config/lazynats/config.yaml
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config dir: %w", err)
	}
	return filepath.Join(dir, "lazynats", "config.yaml"), nil
}

// Load reads the config via viper: file (if it exists) + LAZYNATS_*
// environment variables. A missing file is not an error; the application starts
// with default values (plus anything overridden via env).
func Load(path string) (Config, error) {
	v := viper.New()

	v.SetDefault("nats_url", Default().NatsURL)
	v.SetDefault("theme_path", "")

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(expandHome(path))
		if err := v.ReadInConfig(); err != nil {
			if !os.IsNotExist(err) {
				if _, isNotFound := err.(viper.ConfigFileNotFoundError); !isNotFound {
					return Config{}, fmt.Errorf("read config %s: %w", path, err)
				}
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.NatsURL == "" {
		cfg.NatsURL = Default().NatsURL
	}

	return cfg, nil
}

// expandHome expands "~/..." into the user's home directory.
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
