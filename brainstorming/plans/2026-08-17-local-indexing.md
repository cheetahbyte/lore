# Local-First Indexing Implementation Plan

> **For agentic workers:** Implement this plan one task at a time. Review each task's deliverable before starting the next. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Lore automatically discover a project's exact dependencies, index richer official/package content, refresh incrementally, and keep the local MCP index current with minimal user setup.

**Architecture:** Add a project-sync layer above the existing source adapters and keep the existing `Store`/`Source` seams. Store source metadata and stable document snapshots in SQLite so refresh can skip unchanged documents; keep FTS5 and optional vectors as the local retrieval backend. Add a lightweight watch command using the standard library instead of a resident service framework.

**Tech Stack:** Go standard library, existing Cobra CLI, existing SQLite/FTS5/vec1 store, existing source adapters.

## Global Constraints

- Keep the static, no-CGO Go binary and existing CLI/MCP behavior working.
- Do not add a dependency for lockfile parsing, archive extraction, HTML parsing, or filesystem watching.
- Preserve explicit `lore add` and manual `lore refresh` compatibility.
- Never silently index an empty source or discard a failed refresh.
- A changed document must be reprocessed; an unchanged document must reuse its prior chunks and embeddings.

### Task 1: Project dependency discovery and sync

**Files:**
- Create: `internal/sync/discover.go`
- Create: `internal/sync/discover_test.go`
- Create: `cmd/lore/sync.go`
- Modify: `cmd/lore/main.go`
- Modify: `README.md`

**Interfaces:**
- `sync.Discover(ctx context.Context, root string) ([]sync.Target, error)` returns canonical typed refs with optional exact versions.
- `sync.Target` contains `Ref string`, `Source string`, and `Version string`.
- `ingest.Pipeline.Add` remains the execution path.

- [ ] Parse the smallest useful set of manifests: `package.json`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `go.mod`, `requirements.txt`, `pyproject.toml`, `Cargo.toml`, and `Cargo.lock`.
- [ ] Prefer lockfile versions over manifest ranges and ignore local/path/workspace dependencies.
- [ ] Deduplicate targets and return deterministic ordering.
- [ ] Add `lore sync [path]` with `--dry-run` and `--refresh`; default path is `.`.
- [ ] Print indexed, skipped, and failed counts without aborting unrelated targets.
- [ ] Test discovery for representative manifests and exact-version selection.

### Task 2: Efficient source fetching and richer content

**Files:**
- Modify: `internal/source/github.go`
- Modify: `internal/source/registries.go`
- Modify: `internal/source/source.go`
- Create: `internal/source/archive.go`
- Create: `internal/source/archive_test.go`

**Interfaces:**
- GitHub and registry adapters continue implementing `source.Source`.
- Archive extraction returns `[]RawPage` and filters by documented file extensions and size.

- [ ] Fetch GitHub version archives once and extract documentation/examples locally.
- [ ] Preserve repository path in `RawPage.Path` and generate stable source URLs.
- [ ] Add package-tarball fallback for npm and PyPI when README/official docs are unavailable.
- [ ] Filter binaries, vendored/generated directories, lockfiles, and oversized files.
- [ ] Use bounded concurrent fetches where an adapter still needs multiple HTTP requests.
- [ ] Test archive path filtering, traversal protection, and representative package contents.

### Task 3: Incremental document snapshots

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/write.go`
- Modify: `internal/store/read.go`
- Create: `internal/store/documents.go`
- Modify: `internal/ingest/ingest.go`
- Create: `internal/store/documents_test.go`

**Interfaces:**
- Add `Document` metadata with URL/path, raw and normalized hashes, validators, parser/chunker versions, and fetch time.
- Add store operations for listing/upserting/deleting document snapshots and replacing chunks for one document.

- [ ] Create a `documents` table keyed by tenant, library, version, and canonical path/URL.
- [ ] Record ETag, Last-Modified, commit/version source metadata where available.
- [ ] Rework ingestion to diff fetched pages by stable document key.
- [ ] Reuse unchanged document chunks and embeddings.
- [ ] Delete chunks for removed documents only.
- [ ] Keep a single transaction for document/chunk replacement where SQLite permits it; fail closed on partial vector updates.
- [ ] Test unchanged-page reuse, changed-page replacement, and removed-page cleanup.

### Task 4: Better chunking and retrieval

**Files:**
- Modify: `internal/chunk/chunk.go`
- Modify: `internal/chunk/split.go`
- Modify: `internal/store/search.go`
- Modify: `internal/store/store.go`
- Create: `internal/chunk/quality_test.go`

**Interfaces:**
- Extend `chunk.Chunk` with document title, language, content kind, and source path where available.
- Keep `chunk.Split` compatible; add metadata-aware splitting rather than replacing callers.

- [ ] Preserve heading context on every chunk and retain fenced-code language.
- [ ] Add small prose overlap without splitting code fences.
- [ ] Classify prose, code, example, and API/configuration content.
- [ ] Boost exact identifiers, headings, code examples, and phrase matches in hybrid retrieval.
- [ ] Deduplicate near-identical chunks before returning results.
- [ ] Add focused tests for code/prose boundaries, headings, overlap, and ranking.

### Task 5: Watch mode and user-facing workflow

**Files:**
- Create: `cmd/lore/watch.go`
- Modify: `cmd/lore/main.go`
- Modify: `README.md`

**Interfaces:**
- `lore watch [path]` repeatedly runs sync when supported manifest files change.

- [ ] Use polling with a bounded interval and file modification times; no watcher dependency.
- [ ] Coalesce rapid changes and preserve cancellation behavior.
- [ ] Run one initial sync, then resync only after relevant manifest changes.
- [ ] Document the recommended local MCP setup: `lore watch` separately, `lore serve` over stdio.
- [ ] Add one runnable CLI smoke check for sync/watch argument behavior.

### Verification

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...` with a workspace-local `GOCACHE` if the default cache is inaccessible.
- [ ] Run `go vet ./...`.
- [ ] Exercise `lore sync --dry-run` against the repository itself.
- [ ] Update README usage and explicitly document unsupported manifest edge cases.

