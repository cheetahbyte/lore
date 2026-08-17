---
labels: [wayfinder:map]
---

## Destination

A written technical design spec for **lore**: an open-source, single static Go
binary that ingests documentation from GitHub repos, llms.txt/llms-full.txt,
package registries (pkg.go.dev, npm, PyPI), and crawled documentation
websites, indexes it, and serves it to LLM agents as an MCP server. Usable
standalone/local-first today, with a hosted multi-tenant deployment path kept
open for later (not built now). The spec is detailed enough to hand to a
build effort with no architectural decisions left pending.

Reference prior art (compare against, don't re-derive): [context7](https://github.com/upstash/context7),
[rtfmbro-mcp](https://github.com/marckrenn/rtfmbro-mcp),
[docs-mcp-server](https://github.com/arabold/docs-mcp-server),
[contextmine](https://github.com/mayflower/contextmine).

## Notes

- Domain: Go, the MCP protocol, documentation ingestion, information
  retrieval (chunking / indexing / search).
- Standing constraint: every dependency choice (storage, embeddings,
  parsing, crawling) must have a pure-Go or fully-vendorable path — no
  external services or sidecar runtimes (Node, Python, hosted vector DBs)
  required for local mode. This is the whole reason to build in Go instead
  of using an existing tool, so it overrides convenience elsewhere.
- Project/binary name: `lore` (this repo).
- This repo has no `/grilling` or `/domain-modeling` skill installed. Fall
  back to plain structured conversation with the human in that spirit —
  sharp, one-question-at-a-time, decisions not deliverables.
- Plan, don't do: this map produces the spec, not the implementation. Tickets
  are decisions; building `lore` itself is a separate effort once the map is
  clear.

## Decisions so far

## Not yet specified

- Freshness/refresh strategy for already-indexed docs (staleness detection,
  re-crawl cadence, manual vs background refresh) — depends on how the
  ingestion-source plugin interface represents source identity/versions.
- Auth, rate limiting, and multi-tenancy specifics for the eventual hosted
  mode — depends on what the local design keeps pluggable/stateless.
- Observability/telemetry approach (logs, metrics) for both local and
  eventual hosted operation.
- Retrieval-quality evaluation/testing strategy (how you'd know if chunking
  or ranking changes made results better or worse) — depends on the
  indexing & retrieval architecture being picked first.

## Out of scope

- Building/operating the hosted multi-tenant service itself. The local
  design must keep a hosted path *possible*, but standing it up is a
  separate future effort, not part of this spec.
