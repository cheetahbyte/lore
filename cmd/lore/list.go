package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cheetahbyte/lore/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List everything currently indexed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			libs, err := app.Store.ListLibraries(cmd.Context(), store.LocalTenant)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "LIBRARY\tVERSIONS\tLAST INDEXED")
			for _, l := range libs {
				last := ""
				if !l.LastIndexedAt.IsZero() {
					last = l.LastIndexedAt.Local().Format(time.RFC3339)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", l.LibraryID, strings.Join(l.Versions, ", "), last)
			}
			return w.Flush()
		},
	}
}
