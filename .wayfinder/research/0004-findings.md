# Research: How context7, rtfmbro-mcp, docs-mcp-server, and contextmine chunk, index, and retrieve docs

Resolves the research question in [0004-research-existing-chunking-retrieval](../tickets/0004-research-existing-chunking-retrieval.md).
Feeds ticket 0006 (architecture decision) — this document only surfaces facts, it does not
recommend an approach for `lore`.

Sources: public GitHub repos, READMEs, architecture docs, and source files of each project, plus
two Upstash blog posts for context7 (whose indexing backend is closed-source). Links are inline.

---

## 1. context7 (upstash/context7)

**Status: the indexing/retrieval backend is closed-source.** The public repo
(https://github.com/upstash/context7) only contains the MCP client, CLI, SDKs, and marketing
site. Its README states explicitly:

> "This repository hosts the MCP server's source code. The supporting components — API backend,
> parsing engine, and crawling engine — are private and not part of this repository."

The predecessor repo, `upstash/context7-legacy`, once held real parsing code but has since been
stripped down to a directory of per-library placeholder folders — no source left either.

What Upstash discloses publicly (README, `docs/adding-libraries.mdx`, `docs/library-owners.mdx`,
and two blog posts — [Introducing Context7](https://upstash.com/blog/context7-llmtxt-cursor) and
[Context7 Without Context Bloat](https://upstash.com/blog/new-context7)):

1. **Chunking**: Not disclosed at the mechanical level. What's public: Context7 parses
   `.md`, `.mdx`, `.markdown`, `.rst`, `.txt`, `.ipynb` files from a repo (raw source files are
   skipped if docs exist; otherwise it falls back to LLM-generating docs from source). The
   pipeline is described in five named stages: **Parse** (extract code snippets/examples) →
   **Enrich** (LLM adds short explanations + metadata to each snippet) → **Vectorize** (embed
   for semantic search) → **Rerank** (score for relevance) → **Cache** (serve from Redis). This
   reads as snippet-centric chunking (one chunk ≈ one code example + its LLM-written blurb),
   not heading- or token-based splitting of raw doc text — but the actual split boundaries,
   sizes, and code are not visible.
2. **Index**: A "vector database" is mentioned but never named. Given Context7 is an Upstash
   product, Upstash Vector or Upstash Search are the obvious candidates, but this is inference,
   not a confirmed fact — nothing in the public materials names the specific engine. A recent
   change also added server-side "searching and filtering" so the MCP client no longer receives
   whole-library dumps, cutting average context tokens ~65% (9.7k → 3.3k) and latency ~38%.
3. **Embedding model**: Not disclosed. Necessarily API-based (server-side, hosted service) —
   there's no local/offline mode since indexing itself only happens on Upstash's infrastructure.
4. **Ranking**: Described only as a "proprietary ranking algorithm" / "custom algorithm" that
   reranks vector-search candidates using "fast reranking models." No model name, no formula.
5. **Weak points**:
   - Closed pipeline means self-hosting or auditing the actual retrieval quality is impossible;
     you must trust Upstash's hosted service (there is a "Developer Guide" to run the MCP
     shim locally, but it still calls `mcp.context7.com`).
   - `context7.json`-based scoping is coarse (folder include/exclude, file exclude) — no
     control over chunk size or splitting strategy from the library-owner side.
   - Version handling is described in rtfmbro's README (third-party view, see below) as
     "theoretically possible but complicated in practice," with docs sometimes stale by
     multiple days for high-churn libraries like Next.js.
   - Entirely dependent on hosted infrastructure — the opposite of `lore`'s dependency-free,
     self-hostable, single-binary goal.

---

## 2. rtfmbro-mcp (marckrenn/rtfmbro-mcp)

**Status: does not chunk, index, or embed anything.** This project takes a fundamentally
different approach from the other three: no RAG pipeline at all. It is explicitly pitched as an
alternative to context7's "search a vector index" model.

Also notable: the actual MCP **server source code is not published** in this repo. The GitHub
repo (https://github.com/marckrenn/rtfmbro-mcp) contains only a README, a `CLAUDE.md`/`.cursor`
instruction file, and CI config — the roadmap explicitly lists "Provide rtfmbro source code: Open
source the server codebase" as an unchecked, future item. The server is only usable as a hosted
remote MCP endpoint (`https://rtfmbro.smolosoft.dev/mcp/`).

1. **Chunking**: None. Docs are fetched whole (README files, or specific files via a
   `read_files` tool that supports line-range slicing) at the exact git tag/commit matching the
   pinned dependency version in the user's lockfile.
2. **Index**: None — no persistent search index. There is a local on-disk **cache** keyed by
   repository commit SHA, used only to avoid redundant GitHub API/clone calls, not for search.
3. **Embedding model**: None used.
4. **Ranking/selection**: No retrieval ranking — instead, "agentic discovery." The agent itself
   navigates the docs: `get_documentation_tree` returns a folder structure, the agent picks
   files, `read_files` fetches them (optionally sliced by line range), and
   `search_github_repositories` falls back to GitHub's own search API when the docs live in a
   different repo (e.g. `tailwindlabs/tailwindcss.com` for Tailwind). Selection is entirely
   delegated to the calling LLM's judgment over a directory tree, not to any server-side
   similarity/keyword search.
5. **Weak points**:
   - No search capability at all — "Search Capabilities: Search across documentation corpus" is
     an open (unchecked) roadmap item, so the approach doesn't scale to large doc sets/monorepos
     where an agent can't afford to browse a full tree.
   - Ecosystem-coupled: works by resolving PyPI/npm package metadata to a GitHub repo + git tag;
     packages without a `pyproject`/`package.json` mapping to a public GitHub tag (private
     registries, GitHub Enterprise, docs in a separate un-guessable repo) fall through to a weak
     GitHub-search fallback.
   - Source unavailable — can't be self-hosted or audited/forked as of this writing, which is a
     hard blocker if `lore` wanted to borrow implementation ideas directly from code.
   - The "no indexing" design is nonetheless a genuinely useful data point: it proves a
     zero-embedding, zero-vector-DB approach is viable for at least some doc-retrieval use cases,
     trading recall/scale for perfect freshness and zero infra.

---

## 3. docs-mcp-server (arabold/docs-mcp-server)

The most mature, fully open-source implementation of the four (MIT-licensed, ARCHITECTURE.md +
several `docs/concepts/*.md` design docs). Node.js/TypeScript, SQLite-backed, runs locally as a
single process (or split coordinator/worker for scaling). Explicitly pitches itself as "the
open-source alternative to Context7, Nia, and Ref.Tools."

1. **Chunking — two-phase, structure-aware then size-optimized**:
   - **Phase 1 (semantic splitting)**: content-type-specific splitters preserve document
     structure: `SemanticMarkdownSplitter` splits Markdown by heading hierarchy,
     `JsonDocumentSplitter` splits by JSON property nesting, `TreesitterSourceCodeSplitter`
     splits source code at AST/semantic boundaries (function/class), `TextDocumentSplitter` is
     the line-based fallback for unstructured content.
   - Every chunk carries a `section: { level, path[] }` (heading path) so structure survives
     splitting and search results can be reassembled with parent/sibling/child context.
   - **Phase 2 (size optimization)**: a `GreedySplitter` merges/re-splits phase-1 chunks against
     three **character-count** thresholds — `minChunkSize` (floor, merge below this),
     `preferredChunkSize` (soft target), `maxChunkSize` (hard ceiling) — never token counts (the
     actual token count depends on the embedding model's tokenizer and is not measured).
   - A hard invariant: no chunk boundary may fall inside an open fenced code block (` ``` `/`~~~`),
     even nested inside a list/blockquote; if honoring that would blow `maxChunkSize` the splitter
     accepts the oversize chunk rather than break the fence.
   - Ingestion pipeline: HTML is cleaned (nav/footer/ads stripped) and converted to Markdown
     before splitting; there's also automatic `llms.txt` discovery during web crawls, preferring
     `.md` URL variants when available and negotiating `Accept: text/markdown` so some sites hand
     back clean Markdown directly, skipping HTML conversion entirely.
2. **Index — SQLite, both full-text and vector, in the same file**: a single normalized SQLite
   DB (`libraries` → `versions` → `pages` → `documents`/chunks) using the `sqlite-vec` extension
   for vector similarity and SQLite **FTS5** (Porter stemmer, Unicode61 tokenizer) for full-text,
   maintained via triggers. Embeddings are stored as a BLOB column directly on the chunk row
   (1536-dim by default) — no separate vector-DB service.
3. **Embedding model — optional, multi-provider, includes a local option**: embeddings are
   explicitly optional ("dramatically improves search quality" if enabled, implying FTS-only
   degrades gracefully without one). Supported providers: OpenAI (`text-embedding-3-small`
   default), Google Gemini/Vertex AI, Azure OpenAI, AWS Bedrock, and — notably — any
   OpenAI-API-compatible local endpoint, explicitly documented for **Ollama** and **LM Studio**
   (`OPENAI_API_BASE=http://localhost:11434/v1`). This is the only one of the four with a
   documented, genuinely local/offline embedding path (though it still means running a separate
   Ollama process, not something bundled in the binary). A metadata table tracks the active
   embedding model+dimension so a model/dimension change is detected and the user is prompted
   before existing vectors are invalidated.
4. **Ranking**: hybrid search combining vector similarity (via `sqlite-vec`) and FTS5 keyword
   search, fused with **Reciprocal Rank Fusion (RRF)** with configurable weights. It overfetches
   candidates from both engines before final ranking, and generates FTS queries in "dual mode"
   (exact phrase + keyword) to improve recall. After ranking, a separate reassembly step
   (`DocumentRetrieverService`) groups same-URL results and clusters nearby chunks by
   `sort_order` distance (`maxChunkDistance` config) so a hit doesn't surface as an isolated
   fragment — adjacent chunks from the same section get pulled in and assembled into one
   contiguous result before scoring/sorting the final list.
   - The project ships a dedicated retrieval-quality benchmark (MRR, Recall@k, nDCG@k, Hit@k
     against a labelled qrel set, plus LLM-judged coherence/faithfulness/answerability), run
     weekly / on demand — the only one of the four with a documented, repeatable quality
     evaluation harness.
5. **Weak points**:
   - Not "dependency-free": requires Node.js 22+, native `better-sqlite3` bindings, and
     optionally Playwright/Chromium for JS-rendered pages — a much heavier runtime footprint
     than a static Go binary.
   - Chunk sizing is character-based, not token-based, so actual token counts fed to the
     embedding model vary by tokenizer and aren't tightly controlled.
   - Without any embedding provider configured, search quality is FTS5-only (pure keyword) — the
     semantic layer isn't self-contained/local by default.
   - Distributed mode (separate worker) adds real operational complexity (tRPC, WebSocket event
     bridging) — solving a scaling problem `lore` (single static binary) likely wants to avoid
     needing at all.

---

## 4. contextmine (mayflower/contextmine)

The most feature-heavy of the four — closer to a full "engineering intelligence" platform than a
docs server. Python (FastAPI) + React + PostgreSQL (with `pgvector` and Apache AGE graph
extension) + a Rust crawler (`spider_md`) + a Prefect-based worker for async sync jobs, plus LSP
integration, Tree-sitter symbol extraction, a GraphRAG layer, and a "deep research" agent.
Self-hosted but via Docker Compose/Kubernetes, not a single binary.

1. **Chunking** (`apps/worker/contextmine_worker/chunking.py`): uses **LangChain's**
   `RecursiveCharacterTextSplitter` (default `chunk_size=1500` chars, `chunk_overlap=200` chars,
   separators `["\n\n", "\n", " ", ""]`) plus `MarkdownHeaderTextSplitter`/
   `Language`-aware splitting for ~15 source languages via LangChain's per-language separators
   (Python, JS/TS, Java, Go, Rust, Ruby, PHP, C/C++, C#, Swift, Kotlin, Scala, HTML, Markdown,
   RST, Solidity, Protobuf). A custom pass (`split_markdown_preserving_code_fences`) extracts
   fenced code blocks first and only runs the recursive splitter on the surrounding text,
   re-merging so no fence is ever cut mid-block. Chunk size is character-based, with a fixed
   200-char overlap between adjacent chunks (unlike docs-mcp-server, which doesn't overlap and
   instead merges/preserves structure).
2. **Index**: single PostgreSQL database, both full-text and vector in the `chunks` table — a
   generated `tsv` column (Postgres `tsvector`) for full-text and a `pgvector` `embedding`
   column for similarity. No separate vector-DB service; Apache AGE (graph extension on the same
   Postgres instance) additionally stores a knowledge graph (symbols, call graphs, GraphRAG
   community summaries) used by the "deep research" agent and code-intelligence tools, alongside
   — not instead of — the chunk-level hybrid search.
3. **Embedding model — API-only, no local option**: `contextmine_core/embeddings.py` defines an
   `Embedder` ABC with exactly two real backends: **OpenAI** (`text-embedding-3-small` default,
   1536-dim; also supports `text-embedding-3-large`/3072-dim and `ada-002`) and **Google Gemini**
   (`text-embedding-004`, 768-dim). Both require an API key (`OPENAI_API_KEY` is a *required*
   env var in the README; Gemini is the only listed alternative). There is a `FakeEmbedder` used
   purely for deterministic test fixtures (SHA-256-seeded pseudo-vectors) — not a real local
   embedding path. No Ollama/local-model support, unlike docs-mcp-server.
4. **Ranking** (`packages/core/contextmine_core/search.py`): textbook hybrid search — Postgres
   FTS via `ts_rank_cd(tsv, plainto_tsquery('english', query))` and pgvector cosine distance
   (`embedding <=> query_vector`) run as two independent ranked lists (top 50 each by default),
   fused with **Reciprocal Rank Fusion**, `score = Σ 1/(k + rank_i)` with `k=60` (the same
   textbook RRF constant used by docs-mcp-server) — the code is small and very readable, a clean
   reference implementation if `lore` wants to replicate RRF logic directly. Results are further
   restricted by an access-control join (per-user visible collections) baked directly into the
   SQL.
5. **Weak points**:
   - Heaviest infrastructure footprint of the four by far: Postgres + pgvector + Apache AGE +
     Prefect + a Rust sidecar + GitHub OAuth — the polar opposite of "single dependency-free Go
     binary." Bringing up a dev environment requires Docker, `uv`, Node 20+, and multiple
     services.
   - No local/offline embedding option at all — `OPENAI_API_KEY` is marked *required* even for
     basic operation; Gemini is the only fallback and is still a paid external API.
   - Fixed chunk_size/overlap (1500/200 chars) are LangChain defaults, not clearly tuned to
     this domain; no evidence in the repo of a chunk-size ablation or benchmark (contrast with
     docs-mcp-server's dedicated retrieval-quality benchmark suite).
   - A huge amount of the project's surface area (Architecture Cockpit, C4 diagrams, coverage
     ingestion, LSP-backed research agent) is orthogonal to "index docs, serve chunks to an
     LLM" — useful to know the ceiling of scope creep in this space, but not directly comparable
     chunking/retrieval material.

---

## Synthesis / Comparison

| | context7 | rtfmbro-mcp | docs-mcp-server | contextmine |
|---|---|---|---|---|
| Chunking strategy | Undisclosed (snippet+LLM-blurb, inferred) | None (fetches whole files) | Structure-aware split (headings/AST/JSON) → greedy char-size merge, fence-safe | LangChain recursive char splitter (1500/200 overlap), fence-safe, per-language |
| Index | Undisclosed (likely Upstash Vector/Search — unconfirmed) | None (SHA-keyed file cache only) | SQLite + `sqlite-vec` + FTS5, one file | PostgreSQL + `pgvector` + FTS (`tsvector`) + Apache AGE graph |
| Embedding model | Undisclosed, API-only (hosted) | None | OpenAI / Gemini / Azure / Bedrock **or local Ollama/LM Studio** | OpenAI (default) or Gemini only — no local option |
| Ranking | "Proprietary" rerank model, undisclosed | Agentic browsing (LLM picks files, no ranking) | Vector + FTS5 via RRF, then chunk-cluster reassembly | Vector + FTS via RRF (k=60), textbook formula, readable reference impl |
| Runtime footprint | N/A (hosted SaaS) | N/A (hosted SaaS; server code unreleased) | Node 22+, native SQLite bindings, optional Playwright/Chromium | Postgres+pgvector+AGE, Prefect, Rust sidecar, Docker |
| Self-hostable, dependency-free? | No (closed, hosted only) | No (closed source) | Partially — local process but Node/native-deps heavy | No — multi-service stack |

Cross-cutting observations, not a recommendation for `lore`:

- **RRF is the de facto standard for hybrid ranking** here: both open-source projects
  (docs-mcp-server, contextmine) independently converged on Reciprocal Rank Fusion with k≈60 to
  merge full-text and vector result lists, rather than a learned reranker or weighted-score
  blend. Both keep full-text and vector search on the *same* embedded database engine
  (SQLite+sqlite-vec, Postgres+pgvector) rather than a separate vector-DB service.
- **No project in this set ships a bundled/local embedding model.** Every embedding path shown
  is either a hosted API (OpenAI/Gemini/Azure/Bedrock, always true for context7 and contextmine)
  or requires the user to separately stand up Ollama/LM Studio (docs-mcp-server). None do
  in-process embedding generation, which matters directly for `lore`'s "no external services to
  run it locally" goal.
- **Chunk sizing is char-based everywhere it's disclosed** (docs-mcp-server, contextmine), never
  token-based at split time — token counts are treated as a downstream concern of whichever
  embedding model's tokenizer is in use.
- **Structure preservation (heading/AST paths) and fence-safety (never split inside a code
  fence) show up independently in both open-source projects** — strong signal these are
  load-bearing correctness properties for docs specifically, not incidental choices.
- **The two extremes bracket the design space**: rtfmbro-mcp shows a workable zero-index,
  agentic-navigation approach (trades recall/scale for perfect freshness and zero
  infrastructure); contextmine shows the far end of scope (graph DB, LSP, coverage ingestion,
  deep-research agents) layered on top of the same core RRF hybrid search. docs-mcp-server sits
  in between as the closest existing analog to what a Go rewrite might target functionally.
- **context7's actual chunking/ranking internals are unknowable from the public repo** — any
  comparison to it can only be behavioral (token/latency numbers from their blog posts), not
  architectural.
