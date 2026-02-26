package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/davenicholson-xyz/vista/internal/api"
	"github.com/davenicholson-xyz/vista/internal/config"
	"github.com/davenicholson-xyz/vista/internal/renderer"
	"github.com/davenicholson-xyz/vista/internal/ui"
)

const usage = `Usage: vista <command> [query]

Commands:
  search, s <query>   search by keyword
  top,    t [query]   top-rated wallpapers
  hot,    h [query]   trending wallpapers
  new,    n [query]   newest wallpapers
  random, r [query]   random wallpapers
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var opts api.SearchOptions
	var label string

	switch cmd {
	case "search", "s":
		if len(args) == 0 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(1)
		}
		opts = api.SearchOptions{Query: strings.Join(args, " "), Sorting: "relevance"}
		label = fmt.Sprintf("Searching for %q", opts.Query)
	case "top", "t":
		opts = api.SearchOptions{Query: strings.Join(args, " "), Sorting: "toplist"}
		label = "Fetching top wallpapers"
	case "hot", "h":
		opts = api.SearchOptions{Query: strings.Join(args, " "), Sorting: "hot"}
		label = "Fetching hot wallpapers"
	case "new", "n":
		opts = api.SearchOptions{Query: strings.Join(args, " "), Sorting: "date_added"}
		label = "Fetching new wallpapers"
	case "random", "r":
		opts = api.SearchOptions{Query: strings.Join(args, " "), Sorting: "random"}
		label = "Fetching random wallpapers"
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n%s", cmd, usage)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
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
