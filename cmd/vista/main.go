package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/davenicholson-xyz/vista/internal/api"
	"github.com/davenicholson-xyz/vista/internal/config"
	"github.com/davenicholson-xyz/vista/internal/renderer"
	"github.com/davenicholson-xyz/vista/internal/ui"
)

const usage = `Usage: vista [flags] <command> [query]

Commands:
  search, s <query>   search by keyword
  top,    t [query]   top-rated wallpapers
  hot,    h [query]   trending wallpapers
  new,    n [query]   newest wallpapers
  random, r [query]   random wallpapers

Flags:
  --apikey          Wallhaven API key
  --purity          comma-separated: sfw,sketchy,nsfw
  --categories      comma-separated: general,anime,people
  --min-resolution  minimum resolution e.g. 1920x1080
  --ratios          comma-separated aspect ratios e.g. 16x9,16x10
  --download-dir    directory to save wallpapers
  --script          script to run after setting wallpaper

Flags override values from ~/.config/vista/config.yaml.
`

func main() {
	apikeyFlag      := flag.String("apikey", "", "Wallhaven API key")
	purityFlag      := flag.String("purity", "", "comma-separated: sfw,sketchy,nsfw")
	categoriesFlag  := flag.String("categories", "", "comma-separated: general,anime,people")
	minResFlag      := flag.String("min-resolution", "", "minimum resolution e.g. 1920x1080")
	ratiosFlag      := flag.String("ratios", "", "comma-separated aspect ratios e.g. 16x9,16x10")
	downloadDirFlag := flag.String("download-dir", "", "directory to save wallpapers")
	scriptFlag      := flag.String("script", "", "script to run after setting wallpaper")

	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd  := args[0]
	rest := args[1:]

	var opts  api.SearchOptions
	var label string

	switch cmd {
	case "search", "s":
		if len(rest) == 0 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(1)
		}
		opts  = api.SearchOptions{Query: strings.Join(rest, " "), Sorting: "relevance"}
		label = fmt.Sprintf("Searching for %q", opts.Query)
	case "top", "t":
		opts  = api.SearchOptions{Query: strings.Join(rest, " "), Sorting: "toplist"}
		label = "Fetching top wallpapers"
	case "hot", "h":
		opts  = api.SearchOptions{Query: strings.Join(rest, " "), Sorting: "hot"}
		label = "Fetching hot wallpapers"
	case "new", "n":
		opts  = api.SearchOptions{Query: strings.Join(rest, " "), Sorting: "date_added"}
		label = "Fetching new wallpapers"
	case "random", "r":
		opts  = api.SearchOptions{Query: strings.Join(rest, " "), Sorting: "random"}
		label = "Fetching random wallpapers"
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n%s", cmd, usage)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	// Flags override config file values when explicitly provided.
	if *apikeyFlag != "" {
		cfg.APIKey = *apikeyFlag
	}
	if *purityFlag != "" {
		cfg.Purity = strings.Split(*purityFlag, ",")
	}
	if *categoriesFlag != "" {
		cfg.Categories = strings.Split(*categoriesFlag, ",")
	}
	if *minResFlag != "" {
		cfg.MinResolution = *minResFlag
	}
	if *ratiosFlag != "" {
		cfg.Ratios = strings.Split(*ratiosFlag, ",")
	}
	if *downloadDirFlag != "" {
		cfg.DownloadDir = *downloadDirFlag
	}
	if *scriptFlag != "" {
		cfg.Script = *scriptFlag
	}

	client := &api.Client{
		APIKey:        cfg.APIKey,
		Username:      cfg.Username,
		Purity:        cfg.PurityParam(),
		Categories:    cfg.CategoriesParam(),
		MinResolution: cfg.MinResolution,
		Ratios:        cfg.RatiosParam(),
	}

	fmt.Printf("%s...\n", label)
	wallpapers, meta, err := client.SearchPage(opts, 1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(wallpapers) == 0 {
		fmt.Println("No results found.")
		os.Exit(0)
	}

	fmt.Printf("Found %d wallpapers across %d pages. Loading...\n", meta.Total, meta.LastPage)

	var r renderer.ImageRenderer
	if renderer.IsChafaAvailable() {
		r = &renderer.ChafaRenderer{}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: chafa not found, falling back to placeholder renderer")
		r = &renderer.FallbackRenderer{}
	}

	grid := ui.NewGrid(wallpapers, r, cfg.ResolvedDownloadDir(), cfg.Script, client, opts, meta.LastPage)
	defer grid.Cleanup()

	_, err = grid.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
