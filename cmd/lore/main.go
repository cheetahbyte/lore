// Command lore is the CLI decided in https://github.com/cheetahbyte/lore/issues/3.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lore:", err)
		os.Exit(1)
	}
}

func run() error {
	var verbose bool
	var logFormat string

	root := &cobra.Command{
		Use:           "lore",
		Short:         "A local-first documentation index served to LLM agents over MCP.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
	root.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log output format: text or json")

	ctx := context.Background()
	app, cleanup, err := newApp(ctx, verbose, logFormat)
	if err != nil {
		return err
	}
	defer cleanup()

	root.AddCommand(
		newAddCmd(app),
		newRemoveCmd(app),
		newListCmd(app),
		newRefreshCmd(app),
		newSearchCmd(app),
		newSyncCmd(app),
		newWatchCmd(app),
		newServeCmd(app),
	)

	return root.Execute()
}
