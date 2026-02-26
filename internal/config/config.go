package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey      string   `yaml:"apikey"`
	Username    string   `yaml:"username"`
	Purity      []string `yaml:"purity"`
	Categories  []string `yaml:"categories"`
	DownloadDir string   `yaml:"download_dir"`
	Script      string   `yaml:"script"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Purity:      []string{"sfw"},
		Categories:  []string{"general", "anime", "people"},
		DownloadDir: "~/Pictures/wallpapers",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	path := filepath.Join(home, ".config", "vista", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	if len(cfg.Purity) == 0 {
		cfg.Purity = []string{"sfw"}
	}
	if len(cfg.Categories) == 0 {
		cfg.Categories = []string{"general", "anime", "people"}
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = "~/Pictures/wallpapers"
	}

	return cfg, nil
}

// PurityParam converts the human-readable purity list into the 3-bit string
// the Wallhaven API expects: position 0 = sfw, 1 = sketchy, 2 = nsfw.
func (c *Config) PurityParam() string {
	bits := [3]byte{'0', '0', '0'}
	for _, p := range c.Purity {
		switch p {
		case "sfw":
			bits[0] = '1'
		case "sketchy":
			bits[1] = '1'
		case "nsfw":
			bits[2] = '1'
		}
	}
	return string(bits[:])
}

// CategoriesParam converts the human-readable categories list into the 3-bit
// string the Wallhaven API expects: position 0 = general, 1 = anime, 2 = people.
func (c *Config) CategoriesParam() string {
	bits := [3]byte{'0', '0', '0'}
	for _, cat := range c.Categories {
		switch cat {
		case "general":
			bits[0] = '1'
		case "anime":
			bits[1] = '1'
		case "people":
			bits[2] = '1'
		}
	}
	return string(bits[:])
}

func (c *Config) ResolvedDownloadDir() string {
	if len(c.DownloadDir) >= 2 && c.DownloadDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return c.DownloadDir
		}
		return filepath.Join(home, c.DownloadDir[2:])
	}
	return c.DownloadDir
}
