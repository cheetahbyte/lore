package main

import (
	"fmt"
	"strings"

	"github.com/cheetahbyte/lore/internal/ingest"
	"github.com/cheetahbyte/lore/internal/source"
	"github.com/spf13/cobra"
)

func newAddCmd(app *App) *cobra.Command {
	var depth int
	var include, exclude []string

	cmd := &cobra.Command{
		Use:   "add <ref>[@version]",
		Short: "Add and index a documentation source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			ctx := cmd.Context()

			if strings.HasPrefix(ref, string(identityURLPrefix)) {
				if depth > 0 || len(include) > 0 || len(exclude) > 0 {
					sc := app.Config.Sources[ref]
					if depth > 0 {
						sc.Depth = depth
					}
					if len(include) > 0 {
						sc.Include = include
					}
					if len(exclude) > 0 {
						sc.Exclude = exclude
					}
					app.Config.Sources[ref] = sc
					if err := app.SaveConfig(); err != nil {
						return fmt.Errorf("save config: %w", err)
					}
				}
				if sc, ok := app.Config.Sources[ref]; ok {
					ctx = source.WithCrawlOptions(ctx, source.CrawlOptions{Depth: sc.Depth, Include: sc.Include, Exclude: sc.Exclude})
				}
			}

			pipeline := &ingest.Pipeline{Sources: app.Sources, Store: app.Store, Embedder: app.Embedder, Logger: app.Logger}
			id, version, err := pipeline.Add(ctx, ref)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Indexed %s@%s\n", id, version)
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 0, "crawl depth, url: sources only (default 1)")
	cmd.Flags().StringSliceVar(&include, "include", nil, "include URL substrings, url: sources only")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "exclude URL substrings, url: sources only")
	return cmd
}

const identityURLPrefix = "url:"
