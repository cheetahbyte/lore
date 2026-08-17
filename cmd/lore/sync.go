package main

import (
	"fmt"
	"strings"

	"github.com/cheetahbyte/lore/internal/ingest"
	projectsync "github.com/cheetahbyte/lore/internal/sync"
	"github.com/spf13/cobra"
)

func newSyncCmd(app *App) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync [path]",
		Short: "Discover and index dependencies from a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return runSync(cmd, app, root)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list discovered targets without indexing")
	return cmd
}

func runSync(cmd *cobra.Command, app *App, root string) error {
	targets, err := projectsync.Discover(cmd.Context(), root)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("sync: no supported dependencies found in %s", root)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		for _, target := range targets {
			fmt.Fprintln(cmd.OutOrStdout(), target)
		}
		return nil
	}
	pipeline := &ingest.Pipeline{Sources: app.Sources, Store: app.Store, Embedder: app.Embedder, Logger: app.Logger}
	var failed int
	for _, target := range targets {
		if !strings.HasPrefix(target.Ref, "github:") && !strings.HasPrefix(target.Ref, "npm:") && !strings.HasPrefix(target.Ref, "pypi:") && !strings.HasPrefix(target.Ref, "pkg.go.dev:") {
			continue
		}
		if _, _, err := pipeline.Add(cmd.Context(), target.String()); err != nil {
			failed++
			fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s: %v\n", target, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Indexed %s\n", target)
	}
	if failed > 0 {
		return fmt.Errorf("sync: %d target(s) failed", failed)
	}
	return nil
}
