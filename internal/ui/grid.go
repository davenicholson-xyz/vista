package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

type loadResult struct {
	wallpapers []api.Wallpaper
	thumbPaths []string
	nextPage   int
}

// Grid manages the interactive wallpaper grid.
type Grid struct {
	wallpapers  []api.Wallpaper
	renderer    renderer.ImageRenderer
	downloadDir string
	script      string
	tempDir     string

	cols      int
	cellW     int
	cellH     int
	selected  int
	scrollRow int // first visible grid row (0-indexed)

	// cached rendered images: index -> rendered string
	rendered   map[int]string
	thumbPaths []string

	// draw state — track what was last rendered to enable selective updates
	prevSelected  int
	prevScrollRow int
	prevCount     int

	// pagination / async loading
	client     *api.Client
	searchOpts api.SearchOptions
	nextPage   int
	lastPage   int
	loading    bool
	loadCh     chan loadResult
}

func NewGrid(wallpapers []api.Wallpaper, r renderer.ImageRenderer, downloadDir, script string, client *api.Client, opts api.SearchOptions, lastPage int) *Grid {
	tmp, _ := os.MkdirTemp("", "vista-thumbs-*")
	return &Grid{
		wallpapers:  wallpapers,
		thumbPaths:  make([]string, len(wallpapers)),
		renderer:    r,
		downloadDir: downloadDir,
		script:      script,
		tempDir:     tmp,
		rendered:      make(map[int]string),
		prevSelected:  -1,
		client:        client,
		searchOpts:  opts,
		nextPage:    2,
		lastPage:    lastPage,
		loadCh:      make(chan loadResult, 1),
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

// visibleRows returns how many grid rows fit in the terminal.
func (g *Grid) visibleRows() int {
	_, termH := g.termSize()
	vr := termH / (g.cellH + labelHeight)
	if vr < 1 {
		vr = 1
	}
	return vr
}

// ensureVisible adjusts scrollRow so the selected cell is on screen.
func (g *Grid) ensureVisible() {
	vr := g.visibleRows()
	selectedRow := g.selected / g.cols
	if selectedRow < g.scrollRow {
		g.scrollRow = selectedRow
	} else if selectedRow >= g.scrollRow+vr {
		g.scrollRow = selectedRow - vr + 1
	}
}

// maybeLoadMore fires a background fetch if more pages are available and
// the viewport is close to the end of loaded content.
func (g *Grid) maybeLoadMore() {
	if g.loading || g.nextPage > g.lastPage {
		return
	}
	vr := g.visibleRows()
	loadedRows := (len(g.wallpapers) + g.cols - 1) / g.cols
	selectedRow := g.selected / g.cols
	// Load when: loaded content doesn't fill the screen, or we're within
	// one screenful of the end.
	if loadedRows < vr || selectedRow >= loadedRows-vr {
		g.loading = true
		go g.fetchNextPage()
	}
}

func (g *Grid) fetchNextPage() {
	page := g.nextPage
	wallpapers, _, err := g.client.SearchPage(g.searchOpts, page)
	if err != nil {
		// Skip this page and try the next one next time.
		g.loadCh <- loadResult{nextPage: page + 1}
		return
	}
	thumbPaths := make([]string, len(wallpapers))
	for i, wp := range wallpapers {
		p, _ := wallpaper.Download(wp.Thumbs.Small, g.tempDir)
		thumbPaths[i] = p
	}
	g.loadCh <- loadResult{
		wallpapers: wallpapers,
		thumbPaths: thumbPaths,
		nextPage:   page + 1,
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

	// Pre-download first page thumbnails (blocking)
	g.prefetchThumbs()

	// Read stdin in a goroutine so the main loop can also wait on loadCh.
	inputCh := make(chan []byte, 10)
	go func() {
		buf := make([]byte, 16)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(inputCh)
				return
			}
			tmp := make([]byte, n)
			copy(tmp, buf[:n])
			inputCh <- tmp
		}
	}()

	g.draw()
	g.maybeLoadMore()

	for {
		select {
		case key, ok := <-inputCh:
			if !ok {
				return "", nil
			}
			action := parseKey(key)
			switch action {
			case actionQuit:
				clearScreen()
				return "", nil

			case actionUp:
				if g.selected >= g.cols {
					g.selected -= g.cols
					g.ensureVisible()
				}
			case actionDown:
				if g.selected+g.cols < len(g.wallpapers) {
					g.selected += g.cols
					g.ensureVisible()
				}
			case actionLeft:
				if g.selected > 0 {
					g.selected--
					g.ensureVisible()
				}
			case actionRight:
				if g.selected < len(g.wallpapers)-1 {
					g.selected++
					g.ensureVisible()
				}

			case actionOpen:
				if url := g.wallpapers[g.selected].URL; url != "" {
					openURL(url)
				}

			case actionSelect:
				clearScreen()
				term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Print("\033[?25h")

				wp := g.wallpapers[g.selected]
				fmt.Printf("Applying %s...\n", wp.ID)
				path, err := wallpaper.Download(wp.Path, g.downloadDir)
				if err != nil {
					return "", fmt.Errorf("downloading wallpaper: %w", err)
				}
				fmt.Printf("Setting wallpaper: %s\n", path)
				if err := wallpaper.Set(path, g.script); err != nil {
					return "", fmt.Errorf("setting wallpaper: %w", err)
				}
				fmt.Println("Wallpaper set!")
				return path, nil
			}

		case result := <-g.loadCh:
			g.loading = false
			g.wallpapers = append(g.wallpapers, result.wallpapers...)
			g.thumbPaths = append(g.thumbPaths, result.thumbPaths...)
			g.nextPage = result.nextPage
		}

		g.draw()
		g.maybeLoadMore()
	}
}

func (g *Grid) prefetchThumbs() {
	for i, wp := range g.wallpapers {
		if g.thumbPaths[i] == "" {
			p, _ := wallpaper.Download(wp.Thumbs.Small, g.tempDir)
			g.thumbPaths[i] = p
		}
	}
}

func (g *Grid) draw() {
	vr := g.visibleRows()

	needFull := g.prevSelected < 0 ||
		g.scrollRow != g.prevScrollRow ||
		len(g.wallpapers) != g.prevCount

	var b strings.Builder

	if needFull {
		// Full repaint: accumulate into a buffer and write in one shot to
		// minimise the visible blank-screen window.
		b.WriteString("\033[H\033[2J")
		for idx := range g.wallpapers {
			g.writeCellTo(&b, idx, vr)
		}
	} else if g.selected != g.prevSelected {
		// Only the selection changed — repaint just the two affected cells.
		// No screen clear, so there is no flash at all.
		g.writeCellTo(&b, g.prevSelected, vr)
		g.writeCellTo(&b, g.selected, vr)
	}

	if b.Len() > 0 {
		// Park cursor below the grid, then flush.
		fmt.Fprintf(&b, "\033[%d;1H", vr*(g.cellH+labelHeight)+1)
		fmt.Print(b.String())
	}

	g.prevSelected = g.selected
	g.prevScrollRow = g.scrollRow
	g.prevCount = len(g.wallpapers)
}

// writeCellTo renders a single cell (image + selection border + label) into b.
// It is a no-op if the cell is outside the current viewport.
func (g *Grid) writeCellTo(b *strings.Builder, idx int, vr int) {
	if idx < 0 || idx >= len(g.wallpapers) {
		return
	}
	row := idx / g.cols
	if row < g.scrollRow || row >= g.scrollRow+vr {
		return
	}
	col := idx % g.cols

	// terminal coordinates are 1-based
	startRow := (row-g.scrollRow)*(g.cellH+labelHeight) + 1
	startCol := col*g.cellW + 1

	thumbPath := ""
	if idx < len(g.thumbPaths) {
		thumbPath = g.thumbPaths[idx]
	}

	// Write the image line by line with explicit cursor positioning.
	// For pixel protocols (kitty/sixel/iterm) the rendered string has no
	// raw newlines, so this reduces to a single write at the cell origin.
	// For symbols/character-art each line must be explicitly positioned.
	imgLines := strings.Split(strings.TrimRight(g.imageStr(idx, thumbPath), "\n"), "\n")
	for i, line := range imgLines {
		fmt.Fprintf(b, "\033[%d;%dH%s", startRow+i, startCol, line)
	}

	// Selection top border — drawn after the image so it always sits on top.
	if idx == g.selected {
		topBar := "╔" + strings.Repeat("═", g.cellW-2) + "╗"
		fmt.Fprintf(b, "\033[%d;%dH\033[1;96m%s\033[0m", startRow, startCol, topBar)
	}

	// Label — always at a fixed offset below the cell origin.
	wp := g.wallpapers[idx]
	fmt.Fprintf(b, "\033[%d;%dH%s", startRow+g.cellH, startCol, g.formatLabel(idx, wp.Resolution))
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
		// ╚═  1920x1080  ═╝  — bottom half of the selection box
		inner := centerPad(resolution, g.cellW-4)
		return "\033[1;96m╚═" + inner + "═╝\033[0m"
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

func openURL(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd /c start"
	default:
		cmd = "xdg-open"
	}
	exec.Command(cmd, url).Start() //nolint:errcheck
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
	actionOpen
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
		case 'o':
			return actionOpen
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

// TempDir returns the temp dir used for thumbnails.
func (g *Grid) TempDir() string {
	return g.tempDir
}
