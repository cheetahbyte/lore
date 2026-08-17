# Research findings: pure-Go / single-binary-friendly storage, FTS, and embeddings

Ticket: [0005-research-pure-go-storage-embeddings.md](../tickets/0005-research-pure-go-storage-embeddings.md)
Feeds into: ticket 0006 (indexing & retrieval architecture) — **this document does not decide anything**, it only surveys what's viable.

Constraint under test for every candidate: does it let `lore` ship as **one static Go binary**, no CGO, no external runtime/service, no native `.so`/`.dylib` sidecar, in local mode?

---

## Part 1: Embeddable storage with full-text search (and optionally vector search)

### 1. `modernc.org/sqlite` (Jan Mercl / cznic — transpiled C→Go SQLite)

- **CGO**: None. This is a genuine transpilation of the SQLite C amalgamation into Go source (via `ccgo`), not a binding. Builds with `CGO_ENABLED=0`.
- **Full-text search**: Murky. There is no clearly-documented "FTS5 just works" story — multiple users report `no such module: fts` errors when porting code from CGO SQLite drivers to this one, historically. It does *not* transpile the FTS5 C extension the way it transpiles core SQLite. What it does offer is a pure-Go **virtual-table (`vtab`) API** (`modernc.org/sqlite/vtab`) that lets you implement your own FTS-like virtual table in Go — real functionality, but "build it yourself," not "call FTS5."
- **Vector search**: Yes, as of v1.47.0 (2026), via a **CGO-free port of `sqlite-vec`** — a joint effort between the sqlite-vec author and Jan Mercl. Blank-import `modernc.org/sqlite/vec` and you get `vec0` virtual tables / `vec_distance_*` functions with no extension-loading step, no CGO. This is a genuinely notable result: brute-force/vector KNN inside pure-Go SQLite.
- **Maturity**: Very mature and widely used (v1.56.0 as of Aug 2026, BSD-3-Clause, used by ent, sqlc-adjacent tooling, etc.). Actively maintained.
- **Single-binary verdict**: Clean — no CGO, no native files. But treat "FTS5" as **not actually available**; only the DIY vtab path or pairing with a separate FTS engine (e.g. Bleve) covers full-text search.

### 2. `ncruces/go-sqlite3` (WASM build of real SQLite, via `wazero`)

- **CGO**: None. Different trick from modernc: this compiles the *actual* SQLite C source to WebAssembly, then runs that WASM binary inside `wazero`, a pure-Go WebAssembly runtime (the same runtime family that makes wasm-based tooling like Envoy/WASM plugins CGO-free elsewhere). The WASM blob is embedded into the Go binary via `go:embed`, so the end result is still one static binary — nothing to unpack or dlopen.
- **Full-text search**: Yes, real FTS5 — compiled into the WASM build with `-DSQLITE_ENABLE_FTS5=1`, exposed as `github.com/ncruces/go-sqlite3/ext/fts5`. Since this is literally upstream SQLite's C code, it's the real FTS5, not a reimplementation.
- **Vector search**: Yes — `asg017/sqlite-vec-go-bindings` ships a WASM build of `sqlite-vec` specifically targeting `ncruces/go-sqlite3`, so the same CGO-free story extends to vector search.
- **Maturity**: Active — v0.35.3 as of Aug 2026, MIT license, ~1.1k GitHub stars, high test coverage claimed, `database/sql`-compatible driver plus a lower-level API.
- **Caveats**: WASM sandboxing means higher memory use per DB connection than a native/transpiled build (the project's own docs flag this). Query throughput is generally reported as somewhat slower than CGO SQLite, though usable for a docs-indexing workload rather than an OLTP hot path.
- **Single-binary verdict**: Probably the strongest overall candidate in this category — real FTS5 *and* real sqlite-vec, zero CGO, zero external files, single static binary, actively maintained.

### 3. `mattn/go-sqlite3` (the traditional CGO binding)

- **CGO**: Required. Standard `cgo` wrapper around the real SQLite C library, statically compiled into the resulting Go binary (no external `.so` needed at *runtime* — the C code becomes part of the binary at build time). Breaks "no CGO" but does not break "one binary end users run"; it does break trivial `GOOS=x GOARCH=y go build` cross-compilation (needs a C toolchain per target, e.g. `zig cc` or per-arch gcc).
- **Full-text search**: Excellent — FTS5 supported natively behind a build tag (`-tags sqlite_fts5` / `#cgo CFLAGS: -DSQLITE_ENABLE_FTS5`). This is the most mature, most battle-tested FTS5 story of any option here.
- **Vector search**: Not built in; would require loading `sqlite-vec` as a C extension, itself another CGO/native-load path.
- **Maturity**: The most widely deployed Go SQLite driver by a wide margin, extremely mature.
- **Single-binary verdict**: Fine for "one binary," bad for "no CGO" and for effortless cross-compilation. Useful as the maturity baseline other options are compared against.

### 4. Bleve (`blevesearch/bleve`)

- **CGO**: None for full-text search. Bleve is a pure-Go, Lucene-inspired inverted-index search engine (default `scorch` storage segments, historically backed by `bbolt`).
- **Full-text search**: Its core competency — mature, feature-rich (facets, highlighting, geo, numeric ranges, fuzzy/prefix/wildcard queries, custom analyzers). This has been a serious, widely-deployed pure-Go FTS engine for a decade.
- **Vector search**: Added in v2.4+, with hybrid FTS+kNN score fusion in v2.5.4 — **but it requires a custom-built `faiss_c` dynamic library** (`blevesearch/faiss`, a modified faiss fork) present on the system, built via cmake, plus a `vectors` Go build tag gated on that library being available. This reintroduces both a C++ toolchain *and* a native shared-library runtime dependency — the exact thing the project is trying to avoid.
- **Maturity**: Very mature, Apache-2.0, long production track record (used by many CLI/desktop tools, e.g. as an embedded search index).
- **Single-binary verdict**: FTS-only usage — clean, pure Go, no issue. Vector search — breaks the constraint significantly ("needs a native .so, needs a C++ build step to produce it").

### 5. Badger (`dgraph-io/badger`)

- **CGO**: None — pure-Go embedded LSM-tree key-value store (RocksDB-like design), used at large scale (Dgraph itself, and others).
- **Full-text / vector search**: Not built in at all — it's a raw KV store. Would need something layered on top (roll your own inverted index, or bolt Bleve/an ANN index on top using Badger only for raw storage).
- **Maturity**: Mature, Apache-2.0, actively maintained.
- **Single-binary verdict**: Clean on the CGO/binary front, but it's a storage primitive, not a search solution — would just be the K/V layer under something else.

### 6. bbolt (`etcd-io/bbolt`)

- **CGO**: None — pure-Go embedded B+tree KV store, the actively-maintained fork of the original BoltDB, used internally by etcd and historically as Bleve's default backing store.
- **Full-text / vector search**: None built in, same story as Badger — a storage primitive.
- **Maturity**: Extremely stable (v1, last published June 2026 per Go package index), very widely used as a dependency of other tools.
- **Single-binary verdict**: Clean, but again a building block, not a search engine on its own.

### 7. Pebble (`cockroachdb/pebble`)

- **CGO**: None — pure-Go LSM-tree KV store, originally built to replace RocksDB inside CockroachDB.
- **Full-text / vector search**: None — another storage primitive, mentioned here for completeness alongside Badger/bbolt as a third pure-Go KV option with a strong pedigree (battle-tested in CockroachDB at scale).
- **Single-binary verdict**: Clean, primitive-only.

### 8. `philippgille/chromem-go`

- **CGO**: None — zero third-party dependencies, pure Go.
- **Vector search**: Its whole purpose — a Chroma-like embeddable vector DB, in-memory with optional on-disk persistence (gob-encoded files). Search is **exhaustive/brute-force cosine similarity**, not ANN — explicitly not aimed at scale (its own benchmarks: ~0.3ms for 1,000 docs, ~40ms for 100,000 docs on a mid-range laptop CPU). For a single-project or modest-corpus doc index, that's plausibly fine; for "index a large monorepo's worth of docs across many projects," it likely isn't.
- **Full-text search**: None — vectors only.
- **Maturity**: Beta, pre-1.0, "under heavy construction," breaking changes possible before 1.0.
- **Single-binary verdict**: Clean on constraints, but immature and scale-limited.

### 9. `coder/hnsw`

- **CGO**: None — pure-Go, in-memory HNSW (approximate nearest neighbor) graph index, from Coder (the remote-dev-environment company).
- **Vector search**: Real ANN indexing (unlike chromem-go's brute force), which scales meaningfully better as corpus size grows. Supports `Export`/`Import` over `io.Reader`/`io.Writer` for persistence.
- **Full-text search**: None — vector index only, would need pairing with an FTS engine (Bleve, or hand-rolled).
- **Maturity**: Younger / smaller community footprint than Bleve or chromem-go; worth a closer look at issue activity and API stability before depending on it, but architecturally it's exactly the right shape (pure-Go ANN as a library, not a service).
- **Single-binary verdict**: Clean on constraints; maturity is the main open question, not the architecture.

### 10. DuckDB (`marcboeker/go-duckdb` / `duckdb/duckdb-go`)

- **CGO**: Required. The Go driver **does** bundle prebuilt static libraries for common platforms (macOS amd64/arm64, Linux amd64/arm64, Windows amd64) so the *end binary* has no external `.so` to ship separately — DuckDB's C++ core gets statically linked in at build time. But `CGO_ENABLED=1` and a C++ toolchain are required to build, and if you want the VSS (vector similarity search, HNSW-based) extension included, you must build DuckDB yourself from source with `BUILD_EXTENSIONS="vss"` and a custom static-library bundle — not something you get by default from `go get`.
- **Full-text search**: DuckDB has its own (less mature, less commonly used) FTS extension, separate from SQLite's FTS5 ecosystem.
- **Vector search**: Yes via the VSS extension (HNSW), but only if you do the custom build described above.
- **Single-binary verdict**: "One binary, no external files" is achievable, but with meaningfully more build friction than any SQLite option — a C++ toolchain, a custom extension build, and CGO, all working against trivial cross-compilation. Mentioned for completeness as the closest thing to a full analytical SQL engine with vector search in the Go ecosystem, not as a low-friction option.

---

## Part 2: Local text embedding generation without a sidecar

### A. ONNX Runtime Go bindings (`yalue/onnxruntime_go` and similar)

- **Requirement**: CGO **and** the `onnxruntime` native shared library (`.so`/`.dylib`/`.dll`) present on disk at a known path at runtime. The binding calls into it; it does not embed it.
- **Single-binary verdict**: Breaks the constraint on two axes at once — CGO at build time, and a native library file that must exist alongside (or be extracted from) the binary at runtime. Workable only if you `go:embed` the platform-specific `.so`, write it to a temp path on first run, and load it from there — functional, but inelegant, requires per-platform vendoring of prebuilt onnxruntime binaries, and adds a disk-write/attack-surface concern.

### B. `onnxruntime-purego` (and similarly `dianlight/gollama.cpp` using `purego` for llama.cpp)

- **Requirement**: Uses `ebitengine/purego` to call the C ABI via `dlopen`/`dlsym` at runtime instead of `cgo` at build time. This means the **Go compiler** doesn't need CGO — you get `CGO_ENABLED=0` builds and easy cross-compilation of the Go binary itself. **But the native shared library (onnxruntime or llama.cpp) still has to exist on the target machine at runtime** — purego changes *how* it's loaded (dynamically, no build-time linking), not *whether* it's needed. Same embed-and-extract-at-startup pattern as (A) applies if you want to avoid a separate install step.
- **Single-binary verdict**: A real improvement for the *build/cross-compile* pain point (no C toolchain needed to produce the Go binary), but it does not, by itself, eliminate the "external native library" dependency at runtime — that's a separate problem you still have to solve (embed + extract, or require it preinstalled).

### C. llama.cpp Go bindings (CGO, e.g. `go-skynet/go-llama.cpp`)

- **Requirement**: Build `libbinding.a` from llama.cpp source (a static archive, not a `.so`), vendor it into your module, `CGO_ENABLED=1` at build time. Because it's statically archived and linked in, **the resulting binary needs nothing extra at runtime** — this is a genuinely self-contained single binary once built, unlike the ONNX `.so` path.
- **Precedent**: **Ollama** does exactly this in production — it ships as effectively a single Go binary per platform with `ggml`/llama.cpp compiled in via CGO. This is real-world proof the pattern works. Caveat: ggml's upstream build system has had churn — a recent llama.cpp change (issue tracked upstream, ~mid-2026) made `ggml` build as a *dynamic* library by default, requiring extra CMake flags/patches to force static linking again. So the pattern is provenly viable but requires active vendoring/build-maintenance effort, not "add a `go.mod` line and forget it."
- **Embedding models available**: This path is well-suited to the small BERT/sentence-transformer-style embedding models distributed as `.gguf` — `nomic-embed-text`, `bge-small/base/large`, `all-MiniLM-L6-v2`, `mxbai-embed-large`, etc. These are commonly tens to a few hundred MB, small enough to embed in a release artifact or fetch on first run.
- **Single-binary verdict**: The most credible "real local embeddings, one binary, no external files" path available today — at the cost of CGO at build time (harder cross-compilation, needs a C/C++ toolchain, needs a maintenance eye on upstream llama.cpp/ggml build changes).

### D. Pure-Go transformer inference

- **Reality check**: This does not meaningfully exist today for production use. `gorgonia` is a real pure-Go tensor/autodiff library, but there's no maintained "load a pretrained BERT/sentence-transformer checkpoint and run it" story built on it — you'd be implementing the forward pass, tokenizer, and weight loading yourself, without the SIMD/BLAS-level optimization that makes ggml/onnxruntime fast, and without a community-tested reference implementation to lean on.
- **`ynqa/wego`** is a real, working pure-Go library — but it *trains* classical word2vec/GloVe/LexVec word embeddings from a corpus you supply. These are non-contextual, pre-transformer-era embeddings, meaningfully lower quality for semantic search/retrieval than a modern sentence-transformer model, and it's a training tool rather than "run a pretrained model."
- **Verdict**: No viable pure-Go path to running a real pretrained embedding model exists today. This is the most clear-cut "not realistic yet" finding in this research.

### E. WASM-compiled inference under `wazero` (speculative / DIY)

- **The idea**: `wazero` is a pure-Go WebAssembly runtime, already proven viable in production for a C codebase via `ncruces/go-sqlite3` (real SQLite compiled to WASM, run with zero CGO, embedded in the binary). `ggml`/llama.cpp does have WASM build targets (used for in-browser demos), so in principle a small embedding model's inference could be compiled to WASM and run inside `wazero` — genuinely zero CGO, zero external native files, single static binary.
- **Status found**: I did not find a mature, off-the-shelf **Go package** that packages an embedding model this way today. The existing WASM builds of llama.cpp/ggml are demo/browser-oriented artifacts, not published importable Go modules with an embeddings API. This is a "credible spike, not an available library" — the cleanest theoretical fit for the constraint, but would be a build-it-yourself effort, likely nontrivial (cross-compiling ggml to WASM, wiring a tokenizer, managing model weights inside the WASM linear memory).

### F. External embedding APIs as an opt-in escape hatch

- **Options**: OpenAI, Voyage AI, Cohere (hosted APIs, need a network call + API key), or a user-run **Ollama** server's `/api/embeddings` endpoint (local, but a separate sidecar process the user installs and runs themselves — not bundled by `lore`).
- **Constraint fit**: Trivially satisfies "no CGO, no bundled native runtime" for `lore`'s own binary, since it's just an HTTP client. But it reintroduces the class of dependency the project explicitly wants to avoid by default — network access, a paid API key, a privacy/data-egress tradeoff, or (for Ollama) a separately-installed local service.
- **Design lever worth naming explicitly** (not a decision — that's 0006's call): full-text search (option 1 or 2 above) can be the **always-available, zero-config, pure-Go baseline** that works with no setup, while semantic/vector search becomes an **opt-in enhancement** activated only if the user configures an embedding API key or points at a running Ollama instance. That sidesteps the "must ship a working embedding model inside a static binary" problem for a first release entirely, without foreclosing a later CGO+llama.cpp (or WASM) local-embeddings path for users who want it and are willing to accept a heavier build.

---

## Synthesis and rough viability ranking

**Storage/FTS/vector — cleanest fit for the constraint, most to least attractive:**

1. **`ncruces/go-sqlite3`** — real FTS5 + real sqlite-vec, zero CGO, zero external files (WASM embedded via `go:embed`), single static binary, actively maintained. The strongest all-around candidate if the workload's query volume tolerates WASM-sandbox overhead (plausible for a docs index, not for a hot OLTP path).
2. **`modernc.org/sqlite`** — zero CGO, mature and heavily used, has a genuine pure-Go `sqlite-vec` port for vectors, but FTS5 itself is not clearly available out of the box (would need the vtab API or pairing with Bleve for text search).
3. **Bleve (FTS only) + a pure-Go vector layer (`coder/hnsw` or `chromem-go`)** — combine Bleve's mature pure-Go FTS with a separate pure-Go vector index rather than Bleve's own vector feature (which drags in faiss/CGO/native `.so`). This composition avoids every native dependency while reusing two solid, independently-pure-Go libraries.
4. **Badger / bbolt / Pebble** — solid pure-Go storage primitives, but not solutions by themselves; relevant only as the K/V layer under something built on top.
5. **`mattn/go-sqlite3`** — most mature FTS5 story by far, but CGO, so it's the fallback if the WASM/transpiled options prove too immature or too slow in practice, not a first choice given the stated constraint.
6. **DuckDB** — most build friction (CGO + C++ toolchain + custom extension build for vectors) for the given payoff; only worth it if `lore` wants real analytical SQL, not just docs indexing.

**Local embeddings — none are "free," ranked by how badly each breaks the constraint:**

1. **External API / optional Ollama sidecar** — doesn't break the binary's own constraints at all, but is a dependency by a different name (network, key, or separate process) and isn't "local embedding generation" in the sense the ticket asks about.
2. **llama.cpp via CGO, statically linked** — the most credible *actual* local-embeddings path, proven in production by Ollama; breaks "no CGO" and complicates cross-compilation, but does not need any external file at runtime once built. Needs ongoing build maintenance as upstream ggml's build system evolves.
3. **`purego`-based ONNX/llama.cpp bindings** — removes CGO from the Go build, but still needs a native shared library at runtime; only as clean as your embed-and-extract-at-startup implementation makes it.
4. **Classic ONNX Runtime CGO bindings** — worst of both worlds (CGO at build time *and* an external `.so` at runtime) unless you build the embed/extract machinery yourself.
5. **WASM-compiled embedding inference under `wazero`** — theoretically the cleanest fit (proven pattern via SQLite), but no ready-made Go library exists today; would be a genuine R&D effort.
6. **Pure-Go transformer inference** — not realistic today; no maintained implementation exists. `wego`-style pure-Go word2vec is real but a materially lower-quality substitute, and a training tool rather than an inference tool.

**Bottom line for 0006 to weigh**: pure-Go/CGO-free **full-text search** is genuinely solved today (two credible options: `ncruces/go-sqlite3` or Bleve, both mature). Pure-Go/CGO-free **vector search over your own storage** is also solved for small-to-medium corpora (`modernc.org/sqlite`'s sqlite-vec port, `ncruces`'s sqlite-vec WASM port, or standalone `coder/hnsw`/`chromem-go`). What is *not* solved is generating the embeddings themselves inside a CGO-free static binary — every local-generation path either needs CGO (llama.cpp, proven via Ollama) or needs a native library present at runtime by some mechanism (ONNX-family, with or without `purego`). The one design move that sidesteps this cleanly is making semantic search optional and API/Ollama-driven, with full-text search as the pure-Go always-on fallback — a real option, not a decision made here.
