package wallpaper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Download fetches the URL to destDir, returning the local file path.
// If rawURL is already an absolute local path it is returned as-is.
func Download(rawURL, destDir string) (string, error) {
	if filepath.IsAbs(rawURL) {
		return rawURL, nil
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("creating download dir: %w", err)
	}

	filename := filepath.Base(rawURL)
	dest := filepath.Join(destDir, filename)

	// skip download if already cached
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}

	resp, err := http.Get(rawURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return dest, nil
}
