package main

import (
	"fmt"

	"github.com/cheetahbyte/lore/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(app *App) *cobra.Command {
	var topK int
	cmd := &cobra.Command{
		Use:   "search <library_id>[@version] <query>",
		Short: "Search an indexed library's docs directly from the CLI (same pipeline as the search_docs MCP tool)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			libraryID, version := splitLibraryVersion(args[0])
			query := args[1]

			req := store.SearchRequest{
				TenantID:  store.LocalTenant,
				LibraryID: libraryID,
				Version:   version,
				Query:     query,
				TopK:      topK,
			}
			if app.Embedder != nil {
				if vecs, err := app.Embedder.Embed(cmd.Context(), []string{query}); err == nil && len(vecs) > 0 {
					req.Vector = vecs[0]
				}
			}

			results, err := app.Store.SearchChunks(cmd.Context(), req)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			for _, r := range results {
				fmt.Fprintf(w, "--- %s (%s) [score %.4f] ---\n%s\n\n", r.Chunk.DocURL, r.Chunk.SectionPath, r.Score, r.Chunk.Content)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topK, "top-k", 5, "maximum number of results")
	return cmd
}
