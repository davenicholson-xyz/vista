package wallpaper

import (
	setwallpaper "github.com/davenicholson-xyz/go-setwallpaper/wallpaper"
)

// Set applies the image at path as the desktop wallpaper.
func Set(path string) error {
	return setwallpaper.Set(path)
}
