---
id: 7
title: "Design the common ingestion-source plugin interface and the document/chunk data model"
labels: [wayfinder:grilling]
status: open
assignee: null
blocked_by: [6]
---

## Question

GitHub repos, llms.txt/llms-full.txt, package registries, and crawled doc
sites are four very different fetch mechanisms that all need to land in the
same shape for indexing. What's the common interface each source-type
adapter implements (fetch, list-available-versions/identities, normalize-
to-document), and what does the resulting document/chunk data model look
like (fields carried per chunk: source, library id, version, section
path/heading trail, url, content, etc.)? Depends on the storage/chunking
decision in ticket 0006 since the data model has to fit what that engine
can index.
