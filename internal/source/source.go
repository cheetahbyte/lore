// Package source implements the Source interface and its four adapters,
// decided in https://github.com/cheetahbyte/lore/issues/8: each adapter
// only resolves identity/versions and fetches raw content — chunking is
// one shared pipeline stage in the ingest package, not reimplemented per
// adapter.
package source

import (
	"context"

	"github.com/cheetahbyte/lore/internal/identity"
)

// RawPage is one fetched, not-yet-chunked document.
type RawPage struct {
	URL         string
	Path        string // file path within a repo, if applicable
	Content     string
	ContentType string // "markdown" | "html" | "text"
}

// Source is implemented once per source type (github, npm, pypi,
// pkg.go.dev, llms-txt, url).
type Source interface {
	Type() identity.SourceType

	// Resolve turns a user-typed reference (e.g. "owner/repo" or
	// "react") into a canonical ID and its available versions. Sources
	// with no version concept (llms-txt, url) always return exactly one
	// implicit version: "".
	Resolve(ctx context.Context, ref string) (identity.ID, []string, error)

	// Fetch retrieves the raw content for id at version.
	Fetch(ctx context.Context, id identity.ID, version string) ([]RawPage, error)
}

// Registry looks up a Source by type.
type Registry map[identity.SourceType]Source

func (r Registry) For(t identity.SourceType) (Source, bool) {
	s, ok := r[t]
	return s, ok
}
