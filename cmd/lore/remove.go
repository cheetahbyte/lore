package main

import (
	"fmt"

	"github.com/cheetahbyte/lore/internal/store"
	"github.com/spf13/cobra"
)

func newRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <library_id>[@version]",
		Short: "Remove an indexed library, or one of its versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			libraryID, version := splitLibraryVersion(args[0])
			if err := app.Store.DeleteLibrary(cmd.Context(), store.LocalTenant, libraryID, version); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", args[0])
			return nil
		},
	}
}
