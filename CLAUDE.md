# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o vista ./cmd/vista   # build binary
./vista search "mountain lake"  # run
go build ./...                  # check all packages compile
```

No test suite yet. No linter configured.

## Architecture

The app fetches wallpapers from the Wallhaven API and displays them as an interactive terminal grid. The user navigates and selects a wallpaper to download and set as the desktop background.

**Data flow:** `main` → `api.Client.Search` → thumbnails downloaded to temp dir → `ui.Grid.Run` (raw terminal, keyboard loop) → on Enter: `wallpaper.Download` + `wallpaper.Set`

### Key design decisions

**Image rendering** is abstracted behind `renderer.ImageRenderer` (Render(path, w, h) → string). `ChafaRenderer` shells out to `chafa`. `detectFormat()` in `renderer.go` maps `$TERM_PROGRAM`/`$TERM` to the right chafa `--format` flag (WezTerm → kitty, iTerm2 → iterm, xterm-kitty → kitty, else auto).

**Grid drawing** uses absolute cursor positioning (`\033[row;colH`) per cell rather than line interleaving. This is critical: Kitty/Sixel protocols emit multi-chunk APC sequences that must be written as a contiguous block from the cell origin — splitting them across repositioned rows corrupts the image.

**Cell dimensions:** `cellW = termWidth / cols`, `cellH = cellW * 9 / 32`. The 9/32 factor accounts for 16:9 wallpaper aspect ratio and the ~0.5 width:height pixel ratio of terminal characters.

**Thumbnail caching:** rendered chafa output is cached in `Grid.rendered map[int]string` for the session. Thumbnail images are downloaded to `os.MkdirTemp` and cleaned up on exit.

### Config

`~/.config/vista/config.yaml` — loaded by `internal/config`. Purity is a `[]string` of human-readable values (`sfw`, `sketchy`, `nsfw`); `Config.PurityParam()` converts to the Wallhaven 3-bit string (`"110"` etc.). Defaults: purity `["sfw"]`, download_dir `~/Pictures/wallpapers`.

### Dependencies

- `golang.org/x/term` — raw mode + terminal size
- `gopkg.in/yaml.v3` — config parsing
- `github.com/davenicholson-xyz/go-setwallpaper/wallpaper` — the subpackage path, not the module root; exported function is `wallpaper.Set(path)`
- `chafa` CLI must be installed (`brew install chafa`)
