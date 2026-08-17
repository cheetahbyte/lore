---
id: 6
title: "Pick the indexing & retrieval architecture"
labels: [wayfinder:grilling]
status: open
assignee: null
blocked_by: [4, 5]
---

## Question

Using the findings from ticket 0004 (how existing tools do it) and ticket
0005 (what's actually viable in a pure-Go single binary), decide: the
storage engine, the chunking strategy, and whether retrieval is full-text
only, vector/embedding-based, or hybrid — and if embeddings are involved,
how they're generated (local model vs optional external API vs skipped
entirely). This is the central technical decision the rest of the spec
hangs off of.
