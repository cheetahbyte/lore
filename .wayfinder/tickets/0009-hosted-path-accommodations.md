---
id: 9
title: "Decide what must stay pluggable/stateless in the local design to keep a hosted multi-tenant deployment possible later"
labels: [wayfinder:grilling]
status: open
assignee: null
blocked_by: [6]
---

## Question

The destination is local-first with a hosted path kept open, but building
the hosted service itself is out of scope for this spec. Given the
indexing/storage architecture picked in ticket 0006, what concretely needs
to be an interface rather than a concrete local implementation (storage
backend, per-tenant namespacing of indexed sources, auth hook points) so
that a future hosted deployment doesn't require re-architecting `lore` from
scratch? Keep this to the seams that must exist now — not a design of the
hosted service.
