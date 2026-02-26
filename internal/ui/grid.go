package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/davenicholson-xyz/vista/internal/api"
	"github.com/davenicholson-xyz/vista/internal/renderer"
	"github.com/davenicholson-xyz/vista/internal/wallpaper"
	"golang.org/x/term"
)

const (
	minCellWidth  = 20 // terminal columns
	minCellHeight = 5  // terminal rows (image portion)
	labelHeight   = 1  // rows for resolution label
)

// Grid manages the interactive wallpaper grid.
type Grid struct {
	wallpapers  []api.Wallpaper
	renderer    renderer.ImageRenderer
	downloadDir string
	tempDir     string

	cols     int
	cellW    int
	cellH    int
	selected int

	// cached rendered images: index -> rendered string
	rendered map[int]string
}

func NewGrid(wallpapers []api.Wallpaper, r renderer.ImageRenderer, downloadDir string) *Grid {
	tmp, _ := os.MkdirTemp("", "vista-thumbs-*")
	return &Grid{
		wallpapers:  wallpapers,
		renderer:    r,
		downloadDir: downloadDir,
		tempDir:     tmp,
		rendered:    make(map[int]string),
	}
}

func (g *Grid) Cleanup() {
	os.RemoveAll(g.tempDir)
}

func (g *Grid) termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80, 24
	}
	return w, h
}

func (g *Grid) layout() {
	w, _ := g.termSize()
	g.cols = w / minCellWidth
	if g.cols < 1 {
		g.cols = 1
	}
	g.cellW = w / g.cols

	// Derive cellH from cellW so thumbnails appear at the correct 16:9 ratio.
	// Terminal characters are ~0.5:1 (width:height) in pixels, so a pixel-correct
	// 16:9 image needs: cellH = cellW × (9/16) × 0.5  →  cellW × 9/32.
	g.cellH = g.cellW * 9 / 32
	if g.cellH < minCellHeight {
		g.cellH = minCellHeight
	}
}

// Run starts the interactive UI. Returns the path of the selected wallpaper
// if the user pressed Enter, or "" if they quit.
func (g *Grid) Run() (string, error) {
	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	g.layout()

	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	// Download thumbnails in background as needed
	thumbPaths := make([]string, len(g.wallpapers))
	thumbErrors := make([]error, len(g.wallpapers))

	// Pre-download all thumbnails (blocking for simplicity on first render)
	g.prefetchThumbs(thumbPaths, thumbErrors)

	// Initial render
	g.draw(thumbPaths)

	buf := make([]byte, 16)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}

		key := buf[:n]
		action := parseKey(key)

		switch action {
		case actionQuit:
			clearScreen()
			return "", nil

		case actionUp:
			if g.selected >= g.cols {
				g.selected -= g.cols
			}
		case actionDown:
			if g.selected+g.cols < len(g.wallpapers) {
				g.selected += g.cols
			}
		case actionLeft:
			if g.selected > 0 {
				g.selected--
			}
		case actionRight:
			if g.selected < len(g.wallpapers)-1 {
				g.selected++
			}

		case actionSelect:
			clearScreen()
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Print("\033[?25h")

			wp := g.wallpapers[g.selected]
			fmt.Printf("Downloading %s (%s)...\n", wp.ID, wp.Resolution)
			path, err := wallpaper.Download(wp.Path, g.downloadDir)
			if err != nil {
				return "", fmt.Errorf("downloading wallpaper: %w", err)
			}
			fmt.Printf("Setting wallpaper: %s\n", path)
			if err := wallpaper.Set(path); err != nil {
				return "", fmt.Errorf("setting wallpaper: %w", err)
			}
			fmt.Println("Wallpaper set!")
			return path, nil
		}

		g.draw(thumbPaths)
	}
}

func (g *Grid) prefetchThumbs(paths []string, errs []error) {
	for i, wp := range g.wallpapers {
		p, err := wallpaper.Download(wp.Thumbs.Small, g.tempDir)
		paths[i] = p
		errs[i] = err
	}
}

func (g *Grid) draw(thumbPaths []string) {
	clearScreen()

	for idx := range g.wallpapers {
		row := idx / g.cols
		col := idx % g.cols

		// terminal coordinates are 1-based
		startRow := row*(g.cellH+labelHeight) + 1
		startCol := col*g.cellW + 1

		// Position the cursor at the cell origin then write the entire image
		// output as a single block. Kitty/Sixel protocols encode images as
		// multi-chunk APC sequences; splitting them across repositioned rows
		// (as a line-by-line approach does) causes each chunk to be treated as
		// a separate image at the wrong position.
		fmt.Printf("\033[%d;%dH", startRow, startCol)
		fmt.Print(g.imageStr(idx, thumbPaths[idx]))

		// Label — always at a fixed offset below the cell origin, regardless
		// of where the image output left the cursor.
		wp := g.wallpapers[idx]
		fmt.Printf("\033[%d;%dH%s", startRow+g.cellH, startCol, g.formatLabel(idx, wp.Resolution))
	}

	// Park cursor below the grid.
	totalRows := ((len(g.wallpapers)+g.cols-1)/g.cols)*(g.cellH+labelHeight) + 1
	fmt.Printf("\033[%d;1H", totalRows)
}

func (g *Grid) imageStr(idx int, thumbPath string) string {
	if thumbPath == "" {
		return placeholderLines(g.cellW, g.cellH)
	}
	if cached, ok := g.rendered[idx]; ok {
		return cached
	}
	rendered, err := g.renderer.Render(thumbPath, g.cellW, g.cellH)
	if err != nil {
		rendered = placeholderLines(g.cellW, g.cellH)
	}
	g.rendered[idx] = rendered
	return rendered
}

func (g *Grid) formatLabel(idx int, resolution string) string {
	if idx == g.selected {
		return "\033[7m " + centerPad(resolution, g.cellW-2) + " \033[0m"
	}
	return " " + centerPad(resolution, g.cellW-2) + " "
}

func placeholderLines(w, h int) string {
	var sb strings.Builder
	for i := 0; i < h; i++ {
		sb.WriteString(strings.Repeat("░", w) + "\n")
	}
	return sb.String()
}

func centerPad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	total := width - len(s)
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}


func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// Key actions
type keyAction int

const (
	actionNone keyAction = iota
	actionUp
	actionDown
	actionLeft
	actionRight
	actionSelect
	actionQuit
)

func parseKey(b []byte) keyAction {
	if len(b) == 0 {
		return actionNone
	}

	// Single byte keys
	if len(b) == 1 {
		switch b[0] {
		case 'q', 3: // q or Ctrl+C
			return actionQuit
		case '\r', '\n':
			return actionSelect
		case 'h':
			return actionLeft
		case 'j':
			return actionDown
		case 'k':
			return actionUp
		case 'l':
			return actionRight
		}
	}

	// Escape sequences
	if len(b) >= 3 && b[0] == '\033' && b[1] == '[' {
		switch b[2] {
		case 'A':
			return actionUp
		case 'B':
			return actionDown
		case 'C':
			return actionRight
		case 'D':
			return actionLeft
		}
	}

	return actionNone
}

// tempDirPath returns the temp dir used for thumbnails.
func (g *Grid) TempDir() string {
	return g.tempDir
}
