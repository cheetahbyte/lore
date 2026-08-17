package config

import (
	"os"
	"path/filepath"
)

// ConfigPath returns $XDG_CONFIG_HOME/lore/config.toml, falling back to
// ~/.config/lore/config.toml, per issue #3.
func ConfigPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lore", "config.toml"), nil
}

// DataPath returns $XDG_DATA_HOME/lore/index.db, falling back to
// ~/.local/share/lore/index.db, per issue #3.
func DataPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "lore", "index.db"), nil
}

// EnsureParentDir creates the parent directory of path if it doesn't exist.
func EnsureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
