---
name: go-structure
description: Use this skill for any change under internal/ or cmd/ (the hearthd Go core). It covers the four layers and their import rules, the doc.go contract convention, which packages are lifted from shhh through the shared module and which are new, why rpc handlers hold no domain logic, the gate order (make ci), and astimate as the ceiling. Not for Swift changes (use swiftui-structure) or documentation wording (use documentation).
---

# Go core structure

Today only `cmd/hearthd` and `scripts/docscheck` exist; the packages below are
the target map. The reasons live in `docs/architecture.md`, section "Go core".

## Layers

Dependencies point down only.

```
cmd/hearthd            assembly only: flags, wiring, signal handling
internal/rpc           Connect handlers: translate proto <-> domain, nothing else
internal/orchestrate   tasks, runs, scheduler, budgets, waits
internal/agent         one bot's turn: loop, tool rounds, retry, compaction
internal/tools  memory  handbook  meter  provider  native  (and secret, skill, framing)
internal/org           domain types and pure functions
internal/store         SQLite, migrations, repositories
```

## Import rules

- `org` imports nothing from hearth. Plain types and pure functions only
  (grant resolution, hierarchy checks, budget arithmetic).
- `store` is the only package that imports the SQLite driver. Repositories
  return `org` types. One writer connection, WAL, append-only migrations.
- `agent` depends on `llm` and three seams it is handed: a stream function, a
  tool executor, an approval gate. It never imports `store`, `tools` or `rpc`.
- `orchestrate` alone creates runs, moves task state and decides who runs next.
  It owns the event bus `rpc` streams to the app.
- `provider` wraps `llm`. Nothing but `agent` and `provider` imports `llm`.
- `native` proxies calendar, contacts, shortcuts and on-device-model calls to
  the app; to `tools` they look like any other tool.
- `rpc` handlers contain no domain logic. A handler with an `if` about the
  domain is a defect: move it to `org`, `orchestrate` or `agent`.
- Every tool call is checked against the calling bot's grants in the core
  before dispatch; act-tier tools go through the approval gate.

## The doc.go contract

Each package has a `doc.go` whose package comment names the responsibility, the
invariants the type system does not hold, and the seams it is handed. There are
no per-package spec files and no approval states. Behaviour that spans packages
is specified in `docs/model.md` and cited from code with
`// See docs/model.md#<heading>`; see the `documentation` skill.

## Lifted and new

Lifted from shhh through the shared module (`agentcore`, working name; hearth
consumes its tag): `agent` (loop, retry, compaction), `meter`, `memory` (with a
per-bot scope seam), `secret`, `skill`. Shapes copied, not code: `store`
migrations and the messages table, `tools` registry, `tools/describe`, the
approval principles. New in hearth: `org`, `orchestrate`, `native`, `provider`,
`handbook`, `framing`, `rpc` (Connect over a Unix socket, not shhh's JSON-RPC).
A fix to a lifted package is made in the shared module, not in a hearth copy.

## Gates

`make ci` runs, in order: `fmt-check vet lint test docs-check astimate`. The
cheap checks fail first. Later, `proto-check` joins it.

astimate is the ceiling: capacity limits are absolute, and `orchestrate` and
`store` are split by responsibility early (`orchestrate/schedule`,
`store/migrate`) rather than late. Run it locally (`make astimate`). When a
subagent cannot run it, the orchestrator runs it afterwards; say so in the
hand-off instead of passing `ASTIMATE=true` silently as evidence.

## Before you finish

- Imports point down; no new edge crosses a layer rule above.
- No domain logic in `rpc`; no SQLite outside `store`; no `llm` outside
  `agent` and `provider`.
- New package has a `doc.go`; cross-package behaviour cites `docs/model.md`.
- Tests follow the `testing` skill.
- `make ci` is green, and astimate has run or is flagged for the orchestrator.
