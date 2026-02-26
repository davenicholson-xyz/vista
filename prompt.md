Build a terminal wallpaper selector CLI tool in Go called `vista`.

## Overview
The app fetches wallpapers from the Wallhaven API and displays them as an
interactive grid in the terminal. The user navigates the grid and selects
a wallpaper to download and set as their desktop wallpaper.

## Command
The only command for now is:
  vista search <query>
Example: vista search "mountain lake"

## Image Rendering
Use Chafa (via subprocess) to render images in the terminal. However,
abstract the renderer behind a Go interface so it can be swapped out in
future without changing the rest of the application:

  type ImageRenderer interface {
    Render(imagePath string, width, height int) (string, error)
  }

Implement a ChafaRenderer that satisfies this interface. Chafa should be
called with flags that allow it to auto-detect the terminal protocol
(Kitty, Sixel, iTerm2, Unicode fallback).

## Grid Layout
- Display results as a grid of thumbnail images
- Auto-fit columns to terminal width
- Define a minimum image cell width (e.g. 20 terminal columns) as a constant
- Calculate columns as: floor(terminalWidth / cellWidth), minimum 1
- Below each thumbnail show the resolution (e.g. 1920x1080)
- Download thumbnails to a temp directory for rendering

## Navigation
- Arrow keys and vim bindings (h/j/k/l) to move between images
- The selected image should be highlighted (e.g. a coloured border or label)
- Enter to download and set the selected wallpaper
- q or Ctrl+C to quit

## Setting Wallpaper
Use the library github.com/davenicholson-xyz/go-setwallpaper to set the
wallpaper after downloading the full resolution image.

## Wallhaven API
- Base URL: https://wallhaven.cc/api/v1/search
- Query param: q=<query>
- Only fetch the first page of results for now
- Relevant response fields per wallpaper: id, url, path (full res URL),
  thumbs.small (thumbnail URL), resolution
- Handle API errors gracefully

## Config File
Optional YAML config file at ~/.config/vista/config.yaml
Supported fields:
  apikey: ""         # Wallhaven API key (required for NSFW content)
  username: ""       # Wallhaven username
  purity: "100"      # 100=SFW, 110=SFW+Sketchy, 111=all
  download_dir: "~/Pictures/wallpapers"  # where to save full res images

The app must work without a config file using sensible defaults. If no
config is present, use purity=100 and download to ~/Pictures/wallpapers.

## Project Structure
Suggested layout:
  cmd/vista/main.go       - entry point, arg parsing
  internal/api/           - Wallhaven API client
  internal/renderer/      - ImageRenderer interface + ChafaRenderer
  internal/config/        - config loading
  internal/ui/            - grid display and keyboard navigation
  internal/wallpaper/     - download + set wallpaper logic

## Additional Notes
- Cross-platform: must work on macOS and Linux
- Use os/exec for Chafa subprocess calls
- Use golang.org/x/term for terminal size detection and raw mode input
- Keep dependencies minimal
- Add a go.mod with module name github.com/davenicholson-xyz/vista
