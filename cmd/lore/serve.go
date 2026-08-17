package main

import (
	"github.com/cheetahbyte/lore/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newServeCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server over stdio (issue #2: stdio only, no HTTP transport in v1)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := mcpserver.New(&mcpserver.Server{Store: app.Store, Embedder: app.Embedder})
			return srv.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}
