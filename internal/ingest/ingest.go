// Package ingest wires the Source adapters (issue #8), the chunker
// (issue #7), the optional embedder (issue #7), and the Store (issue #10)
// together into the `lore add`/`lore refresh` flows (issue #3).
package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/cheetahbyte/lore/internal/chunk"
	"github.com/cheetahbyte/lore/internal/embed"
	"github.com/cheetahbyte/lore/internal/identity"
	"github.com/cheetahbyte/lore/internal/source"
	"github.com/cheetahbyte/lore/internal/store"
)

type Pipeline struct {
	Sources  source.Registry
	Store    store.Store
	Embedder embed.Embedder // nil disables vector search, per issue #7
	Logger   *slog.Logger
}

// Add implements `lore add <ref>[@version]`, issue #3. ref must be a
// typed-prefix identity (e.g. "github:owner/repo") or bare adapter-specific
// reference prefixed the same way; bare-name source inference (issue #3's
// "or a bare name lore attempts to infer a source for") isn't implemented
// yet — callers must give the type prefix explicitly for now.
func (p *Pipeline) Add(ctx context.Context, ref string) (identity.ID, string, error) {
	typePart, adapterRef, pinnedVersion := splitRefAndVersion(ref)
	src, ok := p.Sources.For(identity.SourceType(typePart))
	if !ok {
		return "", "", fmt.Errorf("ingest: unknown source type %q (expected one of github, npm, pypi, pkg.go.dev, llms-txt, url)", typePart)
	}

	id, versions, err := src.Resolve(ctx, adapterRef)
	if err != nil {
		return "", "", fmt.Errorf("ingest: resolve %q: %w", ref, err)
	}

	version := pinnedVersion
	if version == "" {
		version = pickDefaultVersion(versions)
	} else if !slices.Contains(versions, version) {
		return "", "", fmt.Errorf("ingest: version %q not found for %s (available: %v)", version, id, versions)
	}

	if err := p.indexVersion(ctx, src, id, version); err != nil {
		return "", "", err
	}
	return id, version, nil
}

// Refresh implements `lore refresh [<library_id>[@version]]`, issue #3,
// using the manual-trigger + content_hash-diff strategy decided in issue
// #12: re-fetch, and only re-chunk/re-embed pages whose hash changed.
// (The conditional-request optimization from #12 — ETags/commit SHAs — and
// the "registry versions are immutable, only discover new ones" nuance are
// not implemented yet; this always does the content_hash-diff fallback
// path, which is correct, just not the cheapest case in #12's design.)
func (p *Pipeline) Refresh(ctx context.Context, libraryID, version string) error {
	id, err := identity.Parse(libraryID)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	src, ok := p.Sources.For(id.Type())
	if !ok {
		return fmt.Errorf("ingest: unknown source type %q", id.Type())
	}

	targetVersion := version
	if targetVersion == "" {
		v, err := p.Store.LatestVersion(ctx, store.LocalTenant, libraryID)
		if err != nil {
			return fmt.Errorf("ingest: %w", err)
		}
		targetVersion = v
	}
	return p.indexVersion(ctx, src, id, targetVersion)
}

func (p *Pipeline) indexVersion(ctx context.Context, src source.Source, id identity.ID, version string) error {
	pages, err := src.Fetch(ctx, id, version)
	if err != nil {
		return fmt.Errorf("ingest: fetch %s@%s: %w", id, version, err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("ingest: %s@%s resolved but no content was fetched — nothing indexed", id, version)
	}

	existingHashes, err := p.Store.ContentHashes(ctx, store.LocalTenant, id.String(), version)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	if len(existingHashes) > 0 && allUnchanged(pages, existingHashes) {
		p.logger().Info("nothing changed", "library_id", id.String(), "version", version)
		return nil
	}

	// UpsertChunks replaces the whole library+version in one call (see
	// store.Store), so every fetched page has to be (re)chunked here, not
	// just the ones whose content_hash changed — otherwise an unchanged
	// page's prior chunks would simply vanish instead of being kept.
	// content_hash-based skipping of the expensive re-embed step for
	// unchanged pages (the optimization issue #12 actually describes)
	// needs the Store to hand back existing embeddings for reuse, which
	// isn't wired up yet — a worthwhile follow-up, not implemented here.
	now := time.Now()
	var allChunks []chunk.Chunk
	for _, page := range pages {
		hash := contentHash(page.Content)
		pieces := chunk.Split(page.Content, chunk.DefaultTargetSize)
		texts := make([]string, len(pieces))
		for i, piece := range pieces {
			texts[i] = piece.Content
		}
		var embeddings [][]float32
		if p.Embedder != nil && len(texts) > 0 {
			embeddings, err = p.Embedder.Embed(ctx, texts)
			if err != nil {
				p.logger().Warn("embedding failed, indexing without vectors", "doc_url", page.URL, "error", err)
				embeddings = nil
			}
		}

		for i, piece := range pieces {
			c := chunk.Chunk{
				TenantID:    store.LocalTenant,
				LibraryID:   id.String(),
				Version:     version,
				DocURL:      page.URL,
				SectionPath: piece.SectionPath,
				Ordinal:     i,
				Content:     piece.Content,
				ContentHash: hash,
				FetchedAt:   now,
			}
			if embeddings != nil && i < len(embeddings) {
				c.Embedding = embeddings[i]
			}
			allChunks = append(allChunks, c)
		}
	}

	if err := p.Store.UpsertChunks(ctx, store.LocalTenant, id.String(), version, allChunks); err != nil {
		return fmt.Errorf("ingest: store chunks: %w", err)
	}
	p.logger().Info("indexed", "library_id", id.String(), "version", version, "chunks", len(allChunks), "pages", len(pages))
	return nil
}

func (p *Pipeline) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// splitRefAndVersion splits "github:owner/repo@v1.2.3" into
// ("github", "owner/repo", "v1.2.3"), with version empty if absent. "@"
// splits from the right so it doesn't collide with npm scoped package
// names like "npm:@scope/name".
func splitRefAndVersion(ref string) (sourceType, adapterRef, version string) {
	t, rest, _ := strings.Cut(ref, ":")
	if idx := strings.LastIndex(rest, "@"); idx > 0 {
		return t, rest[:idx], rest[idx+1:]
	}
	return t, rest, ""
}

// pickDefaultVersion implements the default-version rule from issue #9.
// Each Source.Resolve implementation is responsible for ordering its
// returned versions with its own best notion of "latest" first — the
// registry's own latest marker (npm's dist-tags, PyPI's info.version, the
// Go module proxy's @latest, GitHub's "latest release") where one exists,
// since that's not always the same as the highest semver-sortable version
// (e.g. npm prerelease/canary builds routinely carry a higher base version
// number than the actual stable release). This function just trusts that
// ordering rather than re-deriving it generically.
func pickDefaultVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}

// allUnchanged reports whether every fetched page's content hash matches
// what's already stored, and the page sets are identical in size — the
// short-circuit that avoids a pointless full re-chunk/re-upsert when a
// refresh finds nothing new.
func allUnchanged(pages []source.RawPage, existingHashes map[string]string) bool {
	if len(pages) != len(existingHashes) {
		return false
	}
	for _, page := range pages {
		if existingHashes[page.URL] != contentHash(page.Content) {
			return false
		}
	}
	return true
}
