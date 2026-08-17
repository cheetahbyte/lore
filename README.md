# lore

A local-first documentation index served to LLM agents over [MCP](https://modelcontextprotocol.io).
Single static Go binary — no Node, no Python, no CGO, no hosted service
required. `lore` ingests docs from GitHub repos, llms.txt sites, package
registries, and crawled websites, indexes them with full-text (and
optionally semantic) search, and serves them to any MCP client.

Comparable to [context7](https://github.com/upstash/context7),
[docs-mcp-server](https://github.com/arabold/docs-mcp-server),
[rtfmbro-mcp](https://github.com/marckrenn/rtfmbro-mcp), and
[contextmine](https://github.com/mayflower/contextmine) — see
[issue #1](https://github.com/cheetahbyte/lore/issues/1) for the full design
spec and the reasoning behind every architectural decision below.

## Install

```
go install github.com/cheetahbyte/lore/cmd/lore@latest
```

Prebuilt cross-platform binaries are published to
[GitHub Releases](https://github.com/cheetahbyte/lore/releases) on tag push.

## Usage

```
lore add github:owner/repo        # index a GitHub repo's docs (latest tag, or default branch)
lore add npm:react                # index an npm package's readme
lore add pypi:requests             # index a PyPI project's description
lore add pkg.go.dev:golang.org/x/tools
lore add llms-txt:example.com      # index a site's llms.txt / llms-full.txt
lore add url:https://docs.example.com --depth 2 --include /docs/

lore list                          # what's indexed
lore search github:owner/repo "how do I configure X"
lore refresh                       # re-fetch everything, skip unchanged content
lore serve                         # run the MCP server over stdio
```

Point any MCP client at `lore serve`. It exposes three tools:
`list_libraries`, `resolve_library`, `search_docs`.

## Config

Global, not per-project — one index at `$XDG_DATA_HOME/lore/index.db`
(`~/.local/share/lore` by default), one config at
`$XDG_CONFIG_HOME/lore/config.toml`:

```toml
[embeddings]
provider = ""   # "" = FTS-only (default), or "openai" | "ollama" to enable vector search
api_key = ""    # or set LORE_EMBEDDINGS_API_KEY
endpoint = ""   # e.g. http://localhost:11434/v1 for Ollama

[sources."url:https://docs.example.com"]
depth = 2
include = ["/docs/"]
```

## Status

Implements the spec in
[issue #1](https://github.com/cheetahbyte/lore/issues/1) end to end: all
four source adapters, structure-aware chunking, FTS5 + optional
vec1-fused-via-RRF retrieval, the three MCP tools, and the full CLI. Known
gaps, tracked as follow-ups rather than blocking:

- Bare-name source inference (`lore add react` without a `npm:` prefix)
  isn't implemented — the type prefix is required for now.
- `lore refresh` always does a full re-fetch + content-hash diff; the
  conditional-request optimization (ETags/commit SHAs) from
  [issue #12](https://github.com/cheetahbyte/lore/issues/12) isn't wired up
  yet, so refreshing unchanged content still costs a fetch, just not a
  re-index.
- The retrieval-quality fixture corpus and CI eval from
  [issue #11](https://github.com/cheetahbyte/lore/issues/11) aren't set up.
