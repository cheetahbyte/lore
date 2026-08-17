# Local-markdown issue tracker

This repo had no issue tracker when its first wayfinder map was charted, so this
is the fallback the wayfinder skill defaults to. It's a plain-markdown stand-in
for a real tracker (GitHub Issues, Linear, ...). If this repo ever gets a real
tracker, migrate `.wayfinder/` into it and delete this doc.

## Layout

```
.wayfinder/
  TRACKER.md          this doc
  map.md               the map issue (there should only ever be one open map)
  tickets/
    NNNN-slug.md        one file per ticket, NNNN = zero-padded sequential id
```

## Map frontmatter

```yaml
---
labels: [wayfinder:map]
---
```

Body follows the shape in the wayfinder skill: `## Destination`, `## Notes`,
`## Decisions so far`, `## Not yet specified`, `## Out of scope`.

## Ticket frontmatter

```yaml
---
id: 1                       # matches the NNNN in the filename
title: "Short ticket title"
labels: [wayfinder:grilling]   # exactly one wayfinder:<type> label: research | prototype | grilling | task
status: open                # open | closed
assignee: null               # null = unclaimed; else the name of whoever claimed it
blocked_by: []                # list of ticket ids that must be status: closed first
---

## Question

...

## Resolution

<!-- added on close: the answer -->
```

## Wayfinding operations

These map the generic operations the wayfinder skill needs onto concrete
actions in this file layout.

- **List the frontier** (open, unblocked, unclaimed children): read every file
  in `tickets/`, keep those with `status: open`, `assignee: null`, and every id
  in `blocked_by` pointing at a ticket file with `status: closed`.
- **Claim a ticket**: set `assignee` in its frontmatter to the name of the
  session/dev doing the work, before starting.
- **Create a ticket**: add a new `tickets/NNNN-slug.md` file with the next
  sequential id, `status: open`, `assignee: null`.
- **Wire blocking**: edit the blocked ticket's frontmatter `blocked_by` list to
  include the id(s) of the tickets that must close first. Blocking is
  one-directional and lives only on the blocked ticket.
- **Resolve a ticket**: append a `## Resolution` section to the ticket body
  with the answer, set `status: closed`, then add one line to the map's
  `## Decisions so far` linking to the ticket file with a one-line gist.
- **Rule a ticket out of scope**: same as resolving, but the map line goes
  under `## Out of scope` instead of `## Decisions so far`, and the body gets
  a `## Out of scope` note instead of `## Resolution` explaining why it's
  beyond the destination.
