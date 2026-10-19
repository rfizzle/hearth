---
name: testing
description: Use this skill when adding, changing, debugging, or running hearth Go tests and quality gates, especially when a test needs the agent loop, a SQLite store, a socket, a subprocess, a real provider, or the native tools. It chooses the hermetic, contract, or integration tier and the fixture pattern that keeps results repeatable. Do not use it for product documentation (use the documentation skill) or for Swift view and store tests (use swiftui-structure).
---

# Testing hearth

Start by classifying the evidence the change needs. A test belongs to exactly
one execution tier; do not make the gate discover a host prerequisite by
failing after it starts.

| Tier | Purpose | Where it runs |
|---|---|---|
| Hermetic | Ordinary behaviour and failure handling | `make test`, the default package tests and `make ci` |
| Contract | A real socket, subprocess or provider boundary | `make test-contract` (build tag `contract`) |
| Integration | The full app-plus-helper launch | `make test-integration` (build tag `integration`) on a macOS runner |

## Hermetic tests

`make test` has no listener, daemon or network. Use `t.TempDir` for files and
`t.Setenv` for process configuration, and a fake at the seam instead of a host
service. The seams hearth gives you:

- **Agent loop.** `agent` is handed a stream function, a tool executor and an
  approval gate. Test it with a fake stream function that replays scripted
  chunks (text, tool calls, a failure class) and a fake tool executor that
  records calls and returns canned results. Never import `store`, `tools` or
  a provider from an `agent` test.
- **Store.** Open a SQLite file in `t.TempDir()` through the same constructor
  production uses, so migrations run. Do not mock the repositories; a fresh
  file is cheap and exercises the real SQL.
- **Native tools.** The core cannot reach EventKit, Contacts or Shortcuts. Test
  `native` proxies against a fake callback channel. Once `hearthd --repl`
  exists, it stubs native tools with canned answers, and orchestration and
  framing scenarios run against that stub without the app.
- **Orchestration.** Drive `orchestrate` with the fake loop and a temp-dir
  store, and assert on task state and the events it emits. Budget arithmetic
  lives in `org` and is tested there as pure functions.
- Do not start an `httptest` server or bind a port in a hermetic test; that is
  a real listener. Pass a fake transport through the package's seam.
- Never `os.Chdir` or `t.Chdir`: it makes the package uncacheable. Pass the
  directory in.
- Do not add `-count=1` to `make test`; it is cacheable on purpose.

Every Make check runs under one environment: no inherited `NO_COLOR`, no
executable Git fsmonitor hook, and every tool cache under `.cache/` (a small
`/tmp` tmpfs fills up on a cold build). When driving Go directly in a
restricted shell, set `GOCACHE` and `GOTMPDIR` to writable directories under
`.cache/` and pass `-mod=readonly`.

## Contract and integration tests

Use a contract test only when the fact being proved is the real boundary: the
Connect socket between app and core, traffic to a real provider, a real
subprocess. Put it in a file with the `contract` build tag — the tag, not an
environment variable, so the file is not compiled into the hermetic tier at
all — and fail clearly when the host cannot provide the prerequisite. Run it
through `make test-contract`. `make vet` reads both tags, so the file still has
to compile in the gate.

The `integration` tag is for launching the app and the helper together on a
macOS runner. A missing prerequisite fails that selected runner; a skip is
never a green substitute for it.

Continuous integration runs the contract and integration targets in dedicated
jobs. Do not add either target to `make ci`.

A component with a live and a fake implementation (a tool executor, the
credential store, a native tool) is a candidate for the `go-conformance-suite`
skill: one suite, run against both.

## Before finishing

1. Run the narrow package test, with caches under `.cache/` when the shell is
   constrained.
2. Run `make docs-check` after changing testing guidance.
3. Run the contract or integration target only when the host can satisfy its
   declared prerequisite.
4. Keep a fixture change and its production seam in the same reviewable
   change; do not hide a real-boundary test in the default suite.
