package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/cheetahbyte/lore/internal/config"
	"github.com/cheetahbyte/lore/internal/embed"
	"github.com/cheetahbyte/lore/internal/identity"
	"github.com/cheetahbyte/lore/internal/logging"
	"github.com/cheetahbyte/lore/internal/source"
	"github.com/cheetahbyte/lore/internal/store"
)

// App holds the shared state every subcommand needs, built once in main
// per the global (not per-project) storage scope decided in issue #3.
type App struct {
	Config     *config.Config
	ConfigPath string
	Logger     *slog.Logger
	Store      store.Store
	Sources    source.Registry
	Embedder   embed.Embedder
}

func newApp(ctx context.Context, verbose bool, logFormat string) (*App, func(), error) {
	logger := logging.New(logging.Options{Verbose: verbose, JSON: logFormat == "json"})
	slog.SetDefault(logger)

	cfgPath, err := config.ConfigPath()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}

	dataPath, err := config.DataPath()
	if err != nil {
		return nil, nil, err
	}
	if err := config.EnsureParentDir(dataPath); err != nil {
		return nil, nil, err
	}
	vectorPath := filepath.Join(filepath.Dir(dataPath), "vectors.db")

	vectorEnabled := cfg.Embeddings.Provider != ""
	st, err := store.Open(ctx, dataPath, vectorPath, vectorEnabled)
	if err != nil {
		return nil, nil, err
	}

	var embedder embed.Embedder
	if vectorEnabled {
		embedder = embed.New(cfg.Embeddings.Endpoint, cfg.EffectiveAPIKey(), "")
	}

	sources := source.Registry{
		identity.GitHub:    source.NewGitHub(),
		identity.NPM:       source.NewNPM(),
		identity.PyPI:      source.NewPyPI(),
		identity.GoPackage: source.NewGoPackage(),
		identity.LLMsTxt:   source.NewLLMsTxt(),
		identity.URL:       source.NewURL(),
	}

	app := &App{
		Config:     cfg,
		ConfigPath: cfgPath,
		Logger:     logger,
		Store:      st,
		Sources:    sources,
		Embedder:   embedder,
	}
	cleanup := func() { st.Close() }
	return app, cleanup, nil
}

func (a *App) SaveConfig() error {
	return config.Save(a.ConfigPath, a.Config)
}

// splitLibraryVersion splits "github:owner/repo@v1.2.3" into
// ("github:owner/repo", "v1.2.3"), version empty if absent.
func splitLibraryVersion(s string) (libraryID, version string) {
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}
