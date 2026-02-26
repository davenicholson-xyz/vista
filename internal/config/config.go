package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey      string `yaml:"apikey"`
	Username    string `yaml:"username"`
	Purity      string `yaml:"purity"`
	DownloadDir string `yaml:"download_dir"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Purity:      "100",
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

	if cfg.Purity == "" {
		cfg.Purity = "100"
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = "~/Pictures/wallpapers"
	}

	return cfg, nil
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
