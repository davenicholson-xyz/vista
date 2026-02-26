package main

import (
	"fmt"
	"os"

	"github.com/davenicholson-xyz/vista/internal/api"
	"github.com/davenicholson-xyz/vista/internal/config"
	"github.com/davenicholson-xyz/vista/internal/renderer"
	"github.com/davenicholson-xyz/vista/internal/ui"
)

func main() {
	if len(os.Args) < 3 || os.Args[1] != "search" {
		fmt.Fprintf(os.Stderr, "Usage: vista search <query>\n")
		os.Exit(1)
	}

	query := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	client := &api.Client{
		APIKey:   cfg.APIKey,
		Username: cfg.Username,
		Purity:   cfg.PurityParam(),
	}

	fmt.Printf("Searching for %q...\n", query)
	wallpapers, meta, err := client.SearchPage(query, 1)
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

	grid := ui.NewGrid(wallpapers, r, cfg.ResolvedDownloadDir(), cfg.Script, client, query, meta.LastPage)
	defer grid.Cleanup()

	_, err = grid.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
