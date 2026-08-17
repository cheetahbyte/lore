// Package config implements lore's minimal config file, decided in
// https://github.com/cheetahbyte/lore/issues/3: only embeddings provider
// settings and per-crawled-source options live here, everything else is
// pure CLI args.
package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Embeddings Embeddings              `toml:"embeddings"`
	Sources    map[string]SourceConfig `toml:"sources"`
}

// Embeddings configures the optional vector-search path decided in issue
// #7. Provider == "" means vector search is disabled and lore stays
// FTS-only.
type Embeddings struct {
	Provider string `toml:"provider"` // "" | "openai" | "ollama"
	APIKey   string `toml:"api_key"`  // LORE_EMBEDDINGS_API_KEY env var overrides this
	Endpoint string `toml:"endpoint"` // e.g. http://localhost:11434 for ollama
}

// SourceConfig holds per-source crawl options, keyed by the source's
// identity string (e.g. "url:https://docs.example.com"). Only meaningful
// for url: (crawled) sources, per issue #3.
type SourceConfig struct {
	Depth   int      `toml:"depth"`
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

// Load reads and parses the config file at path. A missing file is not an
// error — it returns a zero-value Config, since every field has a sensible
// empty default (embeddings disabled, no per-source overrides).
func Load(path string) (*Config, error) {
	cfg := &Config{Sources: map[string]SourceConfig{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.Sources == nil {
		cfg.Sources = map[string]SourceConfig{}
	}
	return cfg, nil
}

// EffectiveAPIKey returns the embeddings API key, letting
// LORE_EMBEDDINGS_API_KEY override whatever's in the config file.
func (c *Config) EffectiveAPIKey() string {
	if v := os.Getenv("LORE_EMBEDDINGS_API_KEY"); v != "" {
		return v
	}
	return c.Embeddings.APIKey
}

// Save writes cfg to path as TOML, creating its parent directory if
// needed.
func Save(path string, cfg *Config) error {
	if err := EnsureParentDir(path); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
