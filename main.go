// Command lazynats is a TUI client for NATS (streams and KV buckets)
// in the spirit of lazygit/lazydocker.
package main

import (
	"fmt"
	"os"
	"time"

	"lazynats/internal/app"
	"lazynats/internal/config"
	"lazynats/internal/natsclient"

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
	fs.StringVarP(&configPath, "config", "c", defaultPath, "path to lazynats config file (yaml)")

	var showVersion bool
	fs.BoolVarP(&showVersion, "version", "v", false, "print version and exit")

	var jwt string
	fs.StringVarP(&jwt, "jwt", "j", "", "jwt token")

	var connectTimeout time.Duration
	fs.DurationVarP(&connectTimeout, "timeout", "t", 5*time.Second, "connect timeout")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Usage:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "lazynats:", err)
		return app.Options{}, false, err
	}

	return app.Options{
		ConfigPath:     configPath,
		ConnectTimeout: connectTimeout,
		Options: natsclient.Options{
			JWT: jwt,
		},
	}, showVersion, nil
}
