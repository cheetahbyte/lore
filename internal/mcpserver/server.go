// Package mcpserver implements the MCP tool surface decided in
// https://github.com/cheetahbyte/lore/issues/2: stdio transport, three
// tools (list_libraries, resolve_library, search_docs), read-only over the
// local index — no tool can trigger ingestion.
package mcpserver

import (
	"context"
	"time"

	"github.com/cheetahbyte/lore/internal/embed"
	"github.com/cheetahbyte/lore/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	Store    store.Store
	Embedder embed.Embedder // nil when vector search is disabled, per issue #7
}

// New builds the MCP server with all three tools registered, ready to
// Run over a Transport (issue #2 decided stdio for v1).
func New(s *Server) *mcp.Server {
	impl := &mcp.Implementation{Name: "lore", Version: "0.1.0"}
	srv := mcp.NewServer(impl, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_libraries",
		Description: "List every library currently indexed locally (lore has no global registry — this is how a client discovers what's actually available, since a client must `lore add` a library via the CLI before it's queryable).",
	}, s.listLibraries)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resolve_library",
		Description: "Resolve a free-text library name (e.g. \"react\" or \"requests\") to canonical library ids among what's indexed locally, along with their available versions.",
	}, s.resolveLibrary)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_docs",
		Description: "Search a specific indexed library's documentation and return ranked, scored chunks (url, section path, content). Use resolve_library first to get a library_id.",
	}, s.searchDocs)

	return srv
}

type libraryOut struct {
	LibraryID     string   `json:"library_id"`
	Versions      []string `json:"versions"`
	LastIndexedAt string   `json:"last_indexed_at,omitempty"`
}

func toLibraryOut(l store.LibraryInfo) libraryOut {
	out := libraryOut{LibraryID: l.LibraryID, Versions: l.Versions}
	if !l.LastIndexedAt.IsZero() {
		out.LastIndexedAt = l.LastIndexedAt.UTC().Format(time.RFC3339)
	}
	return out
}

type listLibrariesInput struct{}

type listLibrariesOutput struct {
	Libraries []libraryOut `json:"libraries"`
}

func (s *Server) listLibraries(ctx context.Context, _ *mcp.CallToolRequest, _ listLibrariesInput) (*mcp.CallToolResult, listLibrariesOutput, error) {
	libs, err := s.Store.ListLibraries(ctx, store.LocalTenant)
	if err != nil {
		return nil, listLibrariesOutput{}, err
	}
	out := listLibrariesOutput{Libraries: make([]libraryOut, len(libs))}
	for i, l := range libs {
		out.Libraries[i] = toLibraryOut(l)
	}
	return nil, out, nil
}

type resolveLibraryInput struct {
	Query string `json:"query" jsonschema:"free-text library name to search for, e.g. 'react' or 'requests'"`
}

type resolveLibraryOutput struct {
	Matches []libraryOut `json:"matches"`
}

func (s *Server) resolveLibrary(ctx context.Context, _ *mcp.CallToolRequest, in resolveLibraryInput) (*mcp.CallToolResult, resolveLibraryOutput, error) {
	libs, err := s.Store.ResolveLibraries(ctx, store.LocalTenant, in.Query)
	if err != nil {
		return nil, resolveLibraryOutput{}, err
	}
	out := resolveLibraryOutput{Matches: make([]libraryOut, len(libs))}
	for i, l := range libs {
		out.Matches[i] = toLibraryOut(l)
	}
	return nil, out, nil
}

type searchDocsInput struct {
	LibraryID string `json:"library_id" jsonschema:"canonical library id from resolve_library, e.g. 'github:owner/repo'"`
	Version   string `json:"version,omitempty" jsonschema:"specific version to search; omit to use the latest"`
	Query     string `json:"query" jsonschema:"search query"`
	TopK      int    `json:"top_k,omitempty" jsonschema:"maximum number of results to return; defaults to 5"`
}

type chunkOut struct {
	URL         string  `json:"url"`
	SectionPath string  `json:"section_path"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
}

type searchDocsOutput struct {
	Results []chunkOut `json:"results"`
}

func (s *Server) searchDocs(ctx context.Context, _ *mcp.CallToolRequest, in searchDocsInput) (*mcp.CallToolResult, searchDocsOutput, error) {
	req := store.SearchRequest{
		TenantID:  store.LocalTenant,
		LibraryID: in.LibraryID,
		Version:   in.Version,
		Query:     in.Query,
		TopK:      in.TopK,
	}
	if s.Embedder != nil {
		if vecs, err := s.Embedder.Embed(ctx, []string{in.Query}); err == nil && len(vecs) > 0 {
			req.Vector = vecs[0]
		}
	}

	results, err := s.Store.SearchChunks(ctx, req)
	if err != nil {
		return nil, searchDocsOutput{}, err
	}
	out := searchDocsOutput{Results: make([]chunkOut, len(results))}
	for i, r := range results {
		out.Results[i] = chunkOut{
			URL:         r.Chunk.DocURL,
			SectionPath: r.Chunk.SectionPath,
			Content:     r.Chunk.Content,
			Score:       r.Score,
		}
	}
	return nil, out, nil
}
