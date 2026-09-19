# lazynats

A TUI for NATS in the style of lazygit / lazydocker, built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture).
Two tabs — **Streams** and **Buckets (KV)** — with a list on the left
(~1/3 of the screen) and content on the right.

![lazynats main window](docs/screenshot.png)

## Stack

- Go 1.23+
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — the whole UI is one Elm-style
  model/update/view loop; this is the core design choice the app is built around
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling; all colors live in `internal/ui/theme`
- [Bubbles](https://github.com/charmbracelet/bubbles) — reusable components (list, viewport, etc.)
- [nats.go](https://github.com/nats-io/nats.go) `v1.45.0` + `jetstream` package (context-aware API) — streams & KV
- [Viper](https://github.com/spf13/viper) — config file + auto-env `LAZYNATS_*`
- [atotto/clipboard](https://github.com/atotto/clipboard) — copy panel content to system clipboard

## Structure

```
main.go                      entrypoint: flags, calls app.Run
internal/config              Config{NatsURL, ThemePath} via viper, Load/Save
internal/natsclient          everything that knows about nats.go: Connect, streams, KV, messages
internal/app                 wiring + bubbletea.Program
internal/ui/theme            Palette + Theme (lipgloss styles), external YAML theme loader
internal/ui/keys             global and list keymaps
internal/ui/contentview      content panel: subject messages / key value + copy to clipboard
internal/ui/streamsview      Streams tab: stream list + subjects (+ message viewer on Enter)
internal/ui/bucketsview      Buckets tab: bucket list + keys (+ value viewer on Enter)
internal/ui/statusbar        bottom bar: connection status + contextual key hints
internal/ui/modal            modal dialogs / confirmations [TODO: уточнить назначение]
internal/ui/root             root model: tabs, layout, routing, key-guard against filter input
```

## Run

```bash
go run . --config ./config.example.yaml
```

Without `--config` the app looks for `~/.config/lazynats/config.yaml`.
If the file is missing it falls back to `nats://127.0.0.1:4222`.

Any config value can be overridden with an environment variable prefixed
with `LAZYNATS_` (viper `AutomaticEnv`):

```bash
LAZYNATS_NATS_URL=nats://prod.example.com:4222 go run .
```

## Building / releases

No prebuilt binaries are kept in the repo. Builds are published via
[GitHub Releases](../../releases) — tag a version (`vX.Y.Z`) and CI
builds cross-platform binaries automatically (e.g. with
[GoReleaser](https://goreleaser.com/)).

Local build:

```bash
go build -o lazynats .
```

## Key bindings

List (Streams / Buckets):
- `tab` — switch tab
- `h` / `l` (or `←` / `→`) — left panel / open right panel
- `j` / `k` (or `↑` / `↓`) — navigate list
- `/` — filter list (built-in from bubbles/list; while filtering global
  hotkeys are disabled)
- `enter` — open content view (same as `l` on the right panel)
- `r` — refresh current tab
- `q` / `ctrl+c` — quit

Content view (after `enter` / `l` on a subject or key):
- `esc` / `h` / `←` — back to list
- `j` / `k`, `pgup` / `pgdn` — scroll (bubbles/viewport)
- `y` — copy all panel content to system clipboard
- `q` — quit

## Theme / customization

All colors are in `internal/ui/theme/theme.go` (`Palette`). To let a
designer tweak colors without touching Go code, drop a file like
`theme.example.yaml` and set the path in config:

```yaml
nats_url: "nats://127.0.0.1:4222"
theme_path: "~/.config/lazynats/theme.yaml"
```
