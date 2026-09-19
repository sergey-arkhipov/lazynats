// Package app wires application dependencies (config, NATS connection, theme)
// and starts the bubbletea program.
package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazynats/internal/config"
	"lazynats/internal/natsclient"
	"lazynats/internal/ui/root"
	"lazynats/internal/ui/theme"
)

// Options holds application startup parameters.
type Options struct {
	ConfigPath string

	// ConnectTimeout is the timeout for the NATS connection attempt.
	// If zero, a default of 10 seconds is used. Exposed primarily for tests.
	ConnectTimeout time.Duration
}

// Run is the entry point for cmd/lazynats/main.go.
func Run(opts Options) error {
	deps, err := Setup(opts)
	if err != nil {
		return err
	}
	defer deps.Client.Close()

	p := tea.NewProgram(deps.Model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

// Dependencies holds wired application dependencies.
// It is returned by Setup so that initialization logic can be unit-tested
// independently of the interactive TUI.
type Dependencies struct {
	Cfg    config.Config
	Theme  theme.Theme
	Client *natsclient.Client
	Model  tea.Model
}

// Setup wires all application dependencies (config, theme, NATS client, root model).
// It is separated from Run so that initialization logic can be unit-tested
// without starting the interactive TUI.
func Setup(opts Options) (*Dependencies, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	th, err := theme.Load(cfg.ThemePath)
	if err != nil {
		return nil, fmt.Errorf("load theme: %w", err)
	}

	timeout := opts.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := natsclient.Connect(ctx, cfg.NatsURL)
	if err != nil {
		return nil, fmt.Errorf("connect to nats %q: %w", cfg.NatsURL, err)
	}

	m := root.New(client, th)

	return &Dependencies{
		Cfg:    cfg,
		Theme:  th,
		Client: client,
		Model:  m,
	}, nil
}
