---
id: 8
title: "Decide how lore handles multiple documentation versions per library"
labels: [wayfinder:grilling]
status: open
assignee: null
blocked_by: [7]
---

## Question

context7 resolves a library id to a specific version's docs. Does `lore` do
the same — and if so, how is version metadata detected/recorded per source
type (a GitHub tag/release vs a registry's version list vs an llms.txt with
no version concept vs a crawled site that's just "current"), how does a
client select a version through the MCP tool surface (ticket 0001), and
what's the default when none is specified? Depends on the per-source data
model from ticket 0007 since that's where version metadata would live.
