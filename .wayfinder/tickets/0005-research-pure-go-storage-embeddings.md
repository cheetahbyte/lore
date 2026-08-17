---
id: 5
title: "Research pure-Go / single-binary-friendly options for local embeddings, vector search, and full-text search"
labels: [wayfinder:research]
status: open
assignee: null
blocked_by: []
---

## Question

Given the standing constraint that `lore` must build/run as a single static
Go binary with no external runtime or service dependency in local mode,
survey what's actually available: pure-Go or CGO-free embeddable storage
with full-text search (e.g. modernc.org/sqlite + FTS5, Bleve, Badger),
options for generating text embeddings locally without a sidecar (e.g.
ONNX-runtime Go bindings, gguf/llama.cpp-via-cgo, or whether local embedding
generation is realistic at all inside a static binary), and whether static
linking is actually achievable for each candidate (note anything that pulls
in CGO or a non-Go dependency, since that breaks the single-binary story).
This feeds directly into [[Pick the indexing & retrieval architecture]]
(ticket 0006) — don't make that decision here, just surface what's viable.
