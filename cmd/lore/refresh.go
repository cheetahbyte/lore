package main

import (
	"context"
	"fmt"

	"github.com/cheetahbyte/lore/internal/ingest"
	"github.com/cheetahbyte/lore/internal/store"
	"github.com/spf13/cobra"
)

func newRefreshCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh [<library_id>[@version]]",
		Short: "Manually re-fetch and re-index already-added sources (issue #12: refresh is always manual, never automatic)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			pipeline := &ingest.Pipeline{Sources: app.Sources, Store: app.Store, Embedder: app.Embedder, Logger: app.Logger}

			if len(args) == 0 {
				return refreshAll(ctx, app, pipeline, cmd)
			}
			libraryID, version := splitLibraryVersion(args[0])
			if version != "" {
				return refreshOne(ctx, pipeline, cmd, libraryID, version)
			}
			return refreshAllVersionsOf(ctx, app, pipeline, cmd, libraryID)
		},
	}
}

func refreshAll(ctx context.Context, app *App, pipeline *ingest.Pipeline, cmd *cobra.Command) error {
	libs, err := app.Store.ListLibraries(ctx, store.LocalTenant)
	if err != nil {
		return err
	}
	for _, lib := range libs {
		if len(lib.Versions) == 0 {
			if err := refreshOne(ctx, pipeline, cmd, lib.LibraryID, ""); err != nil {
				return err
			}
			continue
		}
		for _, v := range lib.Versions {
			if err := refreshOne(ctx, pipeline, cmd, lib.LibraryID, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func refreshAllVersionsOf(ctx context.Context, app *App, pipeline *ingest.Pipeline, cmd *cobra.Command, libraryID string) error {
	libs, err := app.Store.ListLibraries(ctx, store.LocalTenant)
	if err != nil {
		return err
	}
	for _, lib := range libs {
		if lib.LibraryID != libraryID {
			continue
		}
		if len(lib.Versions) == 0 {
			return refreshOne(ctx, pipeline, cmd, libraryID, "")
		}
		for _, v := range lib.Versions {
			if err := refreshOne(ctx, pipeline, cmd, libraryID, v); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("refresh: %q is not indexed", libraryID)
}

func refreshOne(ctx context.Context, pipeline *ingest.Pipeline, cmd *cobra.Command, libraryID, version string) error {
	if err := pipeline.Refresh(ctx, libraryID, version); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Refreshed %s@%s\n", libraryID, version)
	return nil
}
