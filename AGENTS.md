# AGENTS.md

`hearth` is a native Mac app where you run a small organisation of AI
employees. It is a SwiftUI app in `App/` over a bundled pure-Go
core, `hearthd`, in `cmd/` and `internal/`, that the app launches and talks to
over loopback with Connect-RPC and a per-launch token. The core owns the org,
the agent loop and the tools; the app is a thin client. Providers come from `github.com/rfizzle/llm`.
The design is in `docs/`; start at `docs/README.md`.

## Scope and restraint

Make the smallest change that is correct, safe and verifiable, following the
local pattern. Don't refactor, rename, reformat, update dependencies, rewrite
tests or expand documentation unless the requested behaviour needs it. A
review reports concrete, evidenced defects, not style preferences or
speculative refactors. A broadly applicable, low-risk fix that prevents
serious harm may be called out separately as a windfall; don't fold it in.

## Documentation

`project-docs` is the standard (the document set, comments, the same-commit
rule); `documentation` is hearth's delta; `sound-patterns` settles style
(default to what the Go stdlib does, measure).

- **Each document answers one question.** `docs/product.md` is what hearth is,
  `docs/model.md` the things and how work flows, `docs/architecture.md` the
  big shapes and why, `docs/decisions.md` what was decided, this file where the
  code is and what will bite you.
- **Documents name no Go symbol; code cites documents.** The reason goes in
  the comment as prose, then the pointer: `// See docs/model.md#<heading>`.
  Cite only product and design decisions, never local mechanics.
- **Headings are anchors.** Renaming one breaks its citations; fix them in the
  same commit. `make docs-check` verifies every citation resolves.
- **Never reference a story, epic, backlog item or the plan** in a comment,
  document or test name. `make docs-check` fails on a work-item identifier.
- **Nothing in `docs/research/` is cited from code**, and it is never edited.

## Commands

| Task | Command |
|------|---------|
| Build the helper | `make build` |
| Test one package | `go test ./internal/<pkg>` |
| Hermetic tests | `make test` |
| Format | `make fmt` |
| Static checks | `make fmt-check vet lint` |
| Docs citations and ids | `make docs-check` |
| Regenerate the contract | `make proto` |
| Contract lint and breaking changes (offline) | `make proto-check` |
| Generated code matches the contract (network) | `make proto-drift` |
| Shared skills match their source | `make skills-check` |
| Refresh shared skills from `../skills` | `make skills-sync` |
| Contract / integration tests | `make test-contract` / `make test-integration` |
| CI pipeline | `make ci` |
| Everything before a push | `make prepush` |
| Quality ceiling | `make astimate` |
| Known vulnerabilities | `make vulncheck` |
| Install the opt-in pre-push hook | `make hooks` |
| Swift gates | `make ci-swift` |
| Build Hearth.app | `make app-build` |
| Sign the bundle | `make app-sign` |
| Notarise and staple | `make notarize` |
| Check signatures | `make app-verify` |
| Format Swift | `make swift-fmt` |
| List every target | `make help` |

Agents run `make ci`; the orchestrator runs `make prepush`, the same checks, at
every checkpoint and before a push. The hook `make hooks` installs runs it; it
is opt-in and `git push --no-verify` skips it.

## Testing

- `make test` is hermetic: no listener, daemon or network. Tests that cross a
  real boundary carry a build tag (`contract`, `integration`) so they never
  compile into it. See `.agents/skills/testing/SKILL.md`.
- Fakes sit at the seams: a stream function and tool executor for the agent
  loop, a temp-dir SQLite file for the store.
- **Never `os.Chdir` or `t.Chdir` in a test**: it makes the package
  uncacheable. Don't add `-count=1` to `make test`.
- Stdlib `testing` only, table-driven, beside the source.

## Version control

- **Never `git add -A`, `git add .` or `git commit -a`.** Name the paths.
- Scope each commit to this session's work, even when other files are dirty.
- Commit on the checked-out branch, `master` included; don't branch first.
- Subject: `type(scope): subject` with type `feat`, `fix`, `docs`, `refactor`,
  `perf`, `test`, `build`, `ci`, `chore` or `revert`; imperative, lower case,
  no trailing period, under 72 characters. A subject that needs "and" is two
  commits. The body says what the diff cannot.
- The `Co-Authored-By` trailer is added by the tool; don't hand-write another.
- The push is the person's. Never push.

These yield to an explicit instruction to do otherwise.

## Architecture

Package doc comments (`doc.go`) and the comments at each trap are the detailed
map. `cmd/hearthd`, `internal/rpc`, `proto/` and `scripts/` exist today; the
rest is the target from `docs/architecture.md`. Skills for each side: `go-structure` and
`swiftui-structure`.

```
cmd/hearthd           assembly, and the --repl harness (exists)
scripts/docscheck     the checks behind make docs-check, vendored (exists)
scripts/skillscheck   the hash check behind make skills-check, vendored (exists)
internal/             dependencies point down only; only rpc/ exists so far
  rpc/                Connect handlers and the token check; generated code under gen/
  orchestrate/        tasks, runs, scheduler, budgets, waits, event bus
  agent/              one bot's turn: loop, tool rounds, retry, compaction
  tools/ memory/ handbook/ meter/ provider/ native/ secret/ skill/ framing/
  org/                domain types, grant resolution, budget math
  store/              SQLite, migrations, repositories
proto/hearth/v1       the contract; generated Go under internal/rpc/gen
```

```
App/                  Package.swift root: the app and gallery targets
  Hearth/             app target: scenes, windows, helper lifecycle, DI
  HearthGallery/      every component and composition in every state
  Packages/
    HearthKit/        generated client, HearthClient, LiveClient, PreviewClient
    HearthDesign/     tokens and primitives
    HearthUI/         components and compositions
    HearthFeatures/   one @Observable store and screens per feature
    HearthNative/     EventKit, Contacts, Shortcuts, Keychain, notifications
```

Swift imports point down: `Hearth` -> `HearthFeatures` -> `HearthUI` ->
`HearthDesign`; `HearthKit` below all but `HearthDesign`; `HearthNative` only
from `Hearth`.

## Invariants

- **Thin client.** The app holds no business state; every fact comes from the
  core and every change is an RPC. If Swift seems to need logic, the proto is
  missing an RPC or an event
  ([`docs/architecture.md`](docs/architecture.md#swift-app)).
- **Every tool call is checked against the calling bot's grants in the core**
  before dispatch, and act-tier tools go through the approval gate.
- **Credentials never enter model context.** Tool results are scrubbed before
  they are appended; the core never writes credentials to disk.
- **`agent` never imports `store`, `tools` or `rpc`**; `store` alone imports
  the SQLite driver; `rpc` handlers hold no domain logic.
- **Nothing in `docs/research/` is cited from code.**
- **`CGO_ENABLED=0`.** The build is pure Go (`modernc.org/sqlite`).

## Gotchas

- **`/tmp` is a small tmpfs.** A cold Go build fills it. Every Make check puts
  its caches under `.cache/` (`GOCACHE`, `GOTMPDIR`); when running Go directly,
  do the same.
- **astimate is run by the orchestrator.** A subagent that cannot run it passes
  `ASTIMATE=true` to `make ci` and says so; the orchestrator runs the real one.
- **`goimports` must be on the `PATH`** for `make fmt-check`; it refuses to run
  without it. Install: `go install golang.org/x/tools/cmd/goimports@latest`.
- **`buf` must be on the `PATH`** for `make proto`, `make proto-check` and
  `make proto-drift`; they refuse to run without it. Install: `go install
  github.com/bufbuild/buf/cmd/buf@v1.73.0`. The generators are remote
  plugins and the BSR rate-limits anonymous generation, so drift is checked
  outside `make ci`: after editing the proto, run `make proto && make
  proto-drift`. `buf registry login` lifts the limit locally.
- **Git has no identity on the VM.** Commit with `git -c user.name=... -c
  user.email=...`, or set `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`,
  `GIT_COMMITTER_NAME` and `GIT_COMMITTER_EMAIL`.
- **`make swift-test` builds each package in its own `.build`**; the first run
  compiles swift-protobuf more than once (minutes), later runs are cached.
- **A package resource (`Bundle.module`) is a `.bundle` beside the
  executable.** `make app-build` copies them into `Contents/Resources`, or
  the app aborts at launch.
- **`HEARTH_HELPER=./hearthd`** lets `swift run --package-path App Hearth`
  find the helper outside a bundle.
- **There is no signing identity on a developer Mac, by design.** `make
  app-sign` ad-hoc signs and says so; the Release workflow is the only place
  a Developer ID build comes from (D18 in `docs/decisions.md`).
- **`make skills-check` fails on a vendored skill edited here.** Skills live
  in `.agents/skills`; `.agents/skills/README.md` says how they are organised
  and which are shared.
