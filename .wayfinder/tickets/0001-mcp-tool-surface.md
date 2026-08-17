---
id: 1
title: "Pick the MCP tool surface and transport(s) lore exposes to clients"
labels: [wayfinder:grilling]
status: open
assignee: null
blocked_by: []
---

## Question

What MCP tools/resources does `lore` expose to a connecting client (e.g. an
analog to context7's `resolve-library-id` + `get-library-docs` pair), what
are their inputs/outputs, and which transport(s) does it support (stdio only,
or also HTTP/SSE for non-local use)? This defines the entire client-facing
contract independent of how ingestion/storage work internally.
