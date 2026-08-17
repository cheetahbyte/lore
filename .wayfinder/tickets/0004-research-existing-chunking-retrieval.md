---
id: 4
title: "Research how context7, docs-mcp-server, rtfmbro-mcp, and contextmine chunk, index, and retrieve documentation"
labels: [wayfinder:research]
status: open
assignee: null
blocked_by: []
---

## Question

For each of the four reference projects, surface: how they split fetched
docs into chunks (by heading, token count, page, something else), what they
index chunks into (vector DB, full-text index, both), what embedding model
(if any) they use and whether it's local or API-based, and how they rank/
select results for a query. Note anything that's clearly a weak point
worth doing differently. This feeds directly into [[Pick the indexing &
retrieval architecture]] (ticket 0006) — don't make that decision here, just
surface the facts.
