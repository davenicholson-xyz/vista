package wallpaper

import (
	"os/exec"
	"strings"

	setwallpaper "github.com/davenicholson-xyz/go-setwallpaper/wallpaper"
)

// Set applies the image at path as the desktop wallpaper.
// If script is non-empty, it is run with path appended as a final argument.
// Otherwise the go-setwallpaper library is used.
func Set(path, script string) error {
	if script != "" {
		parts := strings.Fields(script)
		parts = append(parts, path)
		cmd := exec.Command(parts[0], parts[1:]...)
		return cmd.Run()
	}
	return setwallpaper.Set(path)
}
