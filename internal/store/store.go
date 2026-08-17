// Package store implements the Store interface decided in
// https://github.com/cheetahbyte/lore/issues/10: everything above this
// package (ingestion, retrieval, MCP handlers) talks only to Store, never
// to a concrete database type, so a future hosted backend can be a second
// implementation rather than a rewrite.
package store

import (
	"context"
	"time"

	"github.com/cheetahbyte/lore/internal/chunk"
)

// LocalTenant is the constant tenant_id every row and query uses outside a
// hosted deployment, per issue #10.
const LocalTenant = "local"

// LibraryInfo describes one indexed library, as returned by ListLibraries
// and ResolveLibraries (the list_libraries and resolve_library MCP tools
// from issue #2).
type LibraryInfo struct {
	LibraryID     string
	Versions      []string
	LastIndexedAt time.Time
}

// SearchRequest is one search_docs query (issue #2). Version == "" resolves
// to the highest semver / latest version per issue #9. Vector == nil means
// the caller isn't running vector search (either it's disabled per issue
// #7, or this particular query is FTS-only).
type SearchRequest struct {
	TenantID  string
	LibraryID string
	Version   string
	Query     string
	TopK      int
	Vector    []float32
}

// ScoredChunk is one search_docs result.
type ScoredChunk struct {
	Chunk chunk.Chunk
	Score float64
}

// Store is the storage interface everything above it depends on. The
// SQLite implementation in this package is the only implementation for
// now; a hosted backend would add a second one.
type Store interface {
	// UpsertChunks replaces all chunks for the given library+version with
	// the ones provided (a full re-index of that library+version).
	UpsertChunks(ctx context.Context, tenantID, libraryID, version string, chunks []chunk.Chunk) error
	UpsertDocumentChunks(ctx context.Context, tenantID, libraryID, version, docURL string, chunks []chunk.Chunk) error
	DeleteDocument(ctx context.Context, tenantID, libraryID, version, docURL string) error

	// DeleteLibrary removes a library, or one of its versions if version
	// is non-empty. Mirrors `lore remove`, issue #3.
	DeleteLibrary(ctx context.Context, tenantID, libraryID, version string) error

	// ListLibraries returns everything indexed for tenantID. Mirrors the
	// list_libraries MCP tool and `lore list`, issues #2/#3.
	ListLibraries(ctx context.Context, tenantID string) ([]LibraryInfo, error)

	// ResolveLibraries fuzzy-matches query against indexed library ids.
	// Mirrors the resolve_library MCP tool, issue #2.
	ResolveLibraries(ctx context.Context, tenantID, query string) ([]LibraryInfo, error)

	// LatestVersion returns the version SearchChunks resolves to when
	// SearchRequest.Version is "", per issue #9's default-version rule.
	LatestVersion(ctx context.Context, tenantID, libraryID string) (string, error)

	// SearchChunks runs the retrieval pipeline from issue #7: always FTS5,
	// plus vector search fused via RRF when req.Vector is set.
	SearchChunks(ctx context.Context, req SearchRequest) ([]ScoredChunk, error)

	// ContentHash returns the stored content_hash for every chunk
	// currently indexed under libraryID+version, keyed by doc_url, for the
	// freshness check from issue #12 to diff against.
	ContentHashes(ctx context.Context, tenantID, libraryID, version string) (map[string]string, error)

	Close() error
}
