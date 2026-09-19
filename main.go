// Command lazynats is a TUI client for NATS (streams and KV buckets)
// in the spirit of lazygit/lazydocker.
package main

import (
	"fmt"
	"os"

	"lazynats/internal/app"
	"lazynats/internal/config"

	"github.com/spf13/pflag"
)

// version, commit and date are injected at build time via -ldflags
// (see Makefile / .goreleaser.yaml). Defaults cover `go run` / `go install`
// without explicit ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run wires flag parsing and app execution. It returns an exit code
// so that main() stays trivial and all logic remains testable.
func run(args []string) int {
	opts, showVersion, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazynats:", err)
		return 1
	}

	if showVersion {
		fmt.Printf("lazynats %s (commit %s, built %s)\n", version, commit, date)
		return 0
	}

	if err := app.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "lazynats:", err)
		return 1
	}
	return 0
}

// parseFlags creates a self-contained flag set so that parsing is testable
// without mutating the global flag.CommandLine.
func parseFlags(args []string) (app.Options, bool, error) {
	fs := pflag.NewFlagSet("lazynats", pflag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defaultPath, _ := config.DefaultPath()

	var configPath string
	fs.StringVarP(&configPath, "config", "c", defaultPath, "path to lazynats config file (yaml). Environment variables LAZYNATS_NATS_URL / LAZYNATS_THEME_PATH override file values")

	var showVersion bool
	fs.BoolVarP(&showVersion, "version", "v", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return app.Options{}, false, err
	}

	return app.Options{ConfigPath: configPath}, showVersion, nil
}
