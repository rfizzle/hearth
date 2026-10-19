# hearth · architecture

The big shapes and why: the two-process split, the contract, the Go core's layers, the Swift app's packages and unit taxonomy, and the gates every change passes. Working guidance for day-to-day changes lives in `.agents/skills`, not here, so that it exists in one place.

## Shape and contract

### 1. Shape

Two processes in one app bundle, one contract between them.

```
Hearth.app
├── Contents/MacOS/Hearth            Swift + SwiftUI (AppKit where SwiftUI falls short)
└── Contents/Helpers/hearthd         Go, pure (CGO_ENABLED=0), launched by the app

   Hearth (Swift)                                 hearthd (Go)
   ┌──────────────────────────┐   Unix socket     ┌──────────────────────────────┐
   │ Windows, sidebar, chat,  │ ◄──────────────► │ Org model, bots, conversations│
   │ Inbox, Settings          │  Connect-RPC     │ Agent loop, orchestration     │
   │ Keychain, TCC, EventKit, │  (proto schema)  │ Providers via rfizzle/llm     │
   │ Contacts, Shortcuts,     │                  │ Tool registry + dispatch      │
   │ notifications, Spotlight,│  core → app      │ Memory, handbook, usage       │
   │ App Intents, menu bar    │  callbacks for   │ SQLite (modernc, pure Go)     │
   │ Foundation Models        │  native tools    │ Workspace file I/O            │
   └──────────────────────────┘                  └──────────────────────────────┘
```

**Why a Go core at all.** The provider layer (`github.com/rfizzle/llm`: backends, streaming, tool calling, pricing catalog, failure classes, ChatGPT plan OAuth) already exists in Go and is shared with shhh, and shhh contains a lift-worthy agent loop, meter, memory and skill packages. Rewriting those in Swift would cost more than the bridge and would fork provider behaviour between the two apps. (D1, decided 2026-10-19)

**Why a subprocess and not a linked library.** Crash isolation (a Go panic cannot take the UI down), independent testing of the core from a CLI (`hearthd --repl`, like `shhh serve`), no cgo, no Go/Swift thread coexistence, and a contract that a future iOS companion could speak to a core running elsewhere. gomobile has no first-class macOS path; cgo c-shared makes streaming callbacks painful and couples the runtimes. (D2, decided 2026-10-19)

**Why Connect-RPC over a Unix socket.** One proto file is the contract; generated Swift and Go clients; server-streaming maps directly onto token streaming; it works over a socket with no ports and no TLS. JSON-RPC (shhh's `internal/rpc`) is the fallback if the Swift Connect toolchain proves heavy; the proto still defines the messages. (D3, decided 2026-10-19)

### 2. Responsibilities

| Concern | Lives in | Notes |
|---|---|---|
| Org, bots, teams, conversations, tasks, runs, usage, handbook, notes | core | SQLite is the source of truth. The app holds view models only. |
| Provider calls, streaming, retries | core | llm for calls; the retry loop is lifted from shhh (`internal/agent/retry.go` precedent), since llm only classifies failures. |
| Agent loop | core | Lifted from shhh `internal/agent` (passive state machine with StreamFunc / ToolExecutor / ApprovalGate seams), stripped of the coding-specific classifier, modes and tools. |
| Orchestration (assign, waits, budgets, schedules) | core | New; `model.md#orchestration` is its spec. |
| Tool registry | core | shhh's `Definition{Tool, Execute, Receipt}` shape. Two tiers only: read and act; act tools consult the approval gate. |
| Core-side tools | core | files.read/write/delete (within granted paths handed over by the app as resolved paths), web.fetch, web.search, assign, ask_owner, remember. |
| Native tools | app, invoked by core | calendar.*, reminders.*, contacts.read, shortcuts.run, notifications. The core calls back into the app over the same connection (bidirectional stream, or the app registers as a tool server on a second socket). The app owns the TCC prompts and the EventKit/Contacts/Shortcuts code. |
| Credentials | app (Keychain) → core (memory) | The app reads keys from Keychain and pushes them to the core on connect and on change. The core never writes credentials to disk itself. |
| ChatGPT plan tokens | core via a Store callback | llm's `chatgpt.Open` needs a `Store`; today only a file store exists. hearth needs a Store implemented by callback to the app, which keeps the refresh token in Keychain. This is a small change in llm (make Store an interface). Until then: FileStore in the app's Application Support container, 0600. |
| Security-scoped bookmarks | app | Folder grants outside the Workspace are picked with NSOpenPanel and stored as bookmarks by the app; the app resolves them and tells the core the real path and the grant. Relevant only if a sandboxed build ships. |
| Workspace folder watching | app (FSEvents) → core | The app notifies the core of changes; the core indexes names and sizes only. |
| On-device model | app | Foundation Models for titles, rolling summaries of short histories, and routing hints. Called by the core through the callback channel; the app answers "unavailable" and the core falls back to the Fast tier. |
| Notifications, Inbox badges, menu bar | app | Driven by a core event stream (`Subscribe`). |
| App Intents, Spotlight, Share, Services | app | Thin wrappers over core RPCs. |

### 3. The contract (sketch)

One `hearth.v1` proto package. Services, not exhaustive:

```
OrgService        GetOrg, UpdateOrg, ExportOrg, ImportOrg
BotService        List, Create, Update, Delete, ListTemplates, SetGrants, ListNotes, EditNote
TeamService       List, Create, Update, Delete
ConversationService  List, Get, Messages(stream, paged), Send(returns run id), Cancel
TaskService       List, Get, Assign, Cancel, Retry, ListSchedules, UpsertSchedule
InboxService      List, Decide(approval), Answer(question), Acknowledge
UsageService      Summary(by bot/team/month), SetBudgets
ProviderService   List, SetCredential, ClearCredential, StartChatGPTLogin, Tiers, SetTiers
EventService      Subscribe(stream): message deltas, run state, task state, inbox changes, usage ticks
NativeToolService (served by the app, called by the core): Calendar*, Reminders*, Contacts*, RunShortcut, OnDeviceGenerate, StoreCredential/LoadCredential, Notify
```

Streaming: `Send` returns immediately; the token stream arrives on `Subscribe` as `MessageDelta{conversation, message, text}` so the UI has one place to consume events and reconnects are simple.

Lifecycle: the app launches `hearthd` with a random socket path under the app's container and a one-time token; the core exits when the socket closes. The app restarts it on crash and shows a quiet banner. Version handshake on connect; mismatch is a bug, not a user-facing state, because they ship together.

### 4. Data

- **SQLite** via `modernc.org/sqlite`, single writer connection, WAL, append-only migrations with a `schema_version` table (shhh pattern). Location: `~/Library/Application Support/Hearth/hearth.db`.
- Tables (first cut): `org`, `bots`, `teams`, `team_members`, `grants`, `conversations`, `messages`, `attachments`, `tasks`, `runs`, `usage`, `handbook_pages`, `bot_notes`, `schedules`, `inbox_items`, `providers`, `tiers`, `settings`.
- `messages` follows shhh's `chat_messages` shape (session, seq, role, content, tool_calls JSON, tool_call_id, attachments) plus `author_bot_id`, `kind`, `task_id`, `run_id`, `cost`.
- **Files** live in the Workspace folder; the database stores relative paths and content hashes. Deleting a bot or team never deletes files.
- **Export** is a zip: `org.json` (bots, teams, grants without bookmarks, tiers without credentials), `handbook/*.md`, `templates/*.json`. No conversations, notes, usage or credentials.
- **Backups**: the Workspace is the user's; the database is covered by Time Machine. No hearth-specific backup in v1.

### 5. Security model

- Bots are a *permission* boundary, not a process boundary. Every tool call is checked against the calling bot's grants in the core before dispatch, and act-tier tools go through the approval gate. This is the opposite of Grok Bot's shared-computer design and is a stated differentiator.
- Credentials never enter model context. Tool results are scrubbed for the known key shapes before they are appended (lift shhh's `secret` scrub).
- Web fetch has no cookies, follows redirects with a cap, strips scripts, limits size, and blocks link-local and private ranges.
- Prompt injection from fetched pages and files is treated as untrusted data: the framing says so, and act-tier tools require approval regardless of what a page says. There is no "always allow" for `files.delete` or any Shortcut that the owner did not individually grant.
- The socket is in a user-only directory; the core checks the peer uid and the launch token.
- No telemetry. Crash reports only with an explicit opt-in, via the macOS mechanism.

### 6. Build, sign, ship

- Repo: single repo. Go module at the root (`cmd/hearthd`, `internal/...`, `proto/`), Xcode project under `App/`. The Makefile builds the Go helper for arm64 (and amd64 if Intel is supported, then `lipo`), and an Xcode build phase copies it into `Contents/Helpers`. (D4, decided 2026-10-19)
- Signing inside-out: helper with hardened runtime and timestamp, then the app; `notarytool` + `stapler`. Pure Go needs no JIT entitlements. Check `LC_BUILD_VERSION` with `vtool` in CI.
- **Developer ID, notarised, Sparkle 2 updates** first. A Mac App Store build is a possible later SKU that would lose Shortcuts execution, Mail automation and wide file access, and would need the helper signed with `app-sandbox` + `inherit`. (D5, decided 2026-10-19)
- Licence MIT or Apache-2.0 to keep the App Store option open and to satisfy OpenAI's open-source sign-in programme. (D6, decided 2026-10-19)
- Minimum macOS 26 (Foundation Models, Liquid Glass, current App Intents). Apple Silicon only unless there is a reason to pay for Intel. (D7, decided 2026-10-19)
- CI: GitHub Actions on macOS runners: `make ci` for Go (fmt-check, vet, lint, test, docs-check, astimate ceilings), `xcodebuild test` for Swift, a notarised build on tags.

### 7. What is lifted from shhh and how

| shhh package | Verdict | Plan |
|---|---|---|
| `internal/agent` core (messages, rounds, tool-round split, steering queue, repeat detection, round caps) | generic | Extract into the shared module (D8, decided 2026-10-19). |
| `internal/agent/retry.go` | generic | Lift with it. |
| `internal/meter` | cleanly generic | Lift as is; attribute Origin = (bot, task, run). |
| `internal/memory` | shhh's model | Stays in shhh (D13). hearth's Notes package is new, against `model.md`'s per-bot notes. |
| `internal/skill` | generic | Lift; role templates can be skills in disguise (SKILL.md with frontmatter) which also gives a path to sharing. |
| `internal/storage` migrations + `chat_messages` | pattern | Copy the pattern, not the schema. |
| `internal/approval` principles (standing rules first, two doors, fail closed) | principles | Re-implement for grants; no shell rules. |
| `internal/secret` scrub | generic | Lift. |
| `internal/rpc` | precedent | Read for shape; hearth uses proto. |
| `internal/tools`, `internal/subagent` worktrees, `internal/ui`, `internal/cli` | coding-specific | Do not lift. |

### 8. Alternative considered: pure Swift

A single-language app using Swift for providers (own OpenAI/Anthropic/Gemini clients, own OAuth), SwiftData for storage and Swift concurrency for the loop. It would be simpler to sign, easier to put on the App Store, and more idiomatic. It was set aside because it duplicates llm and the shhh agent core, splits provider behaviour across the two apps, and the owner's intent is for hearth to be the third consumer of the shared Go code. D1 was decided for the Go core on 2026-10-19 (`decisions.md`); the alternative is recorded here for the reasoning.

## Go core

### 1. Layering

Four layers, dependencies point down only. astimate's cross-package duplication rule and golangci's `depguard` enforce the edges.

```
cmd/hearthd                      assembly only: flags, wiring, signal handling
internal/rpc                     Connect handlers; translate proto <-> domain; no logic
internal/orchestrate             tasks, runs, scheduler, budgets, waits (model.md#orchestration)
internal/agent                   one bot's turn: the loop, tool rounds, retry, compaction (lifted from shhh)
internal/tools  internal/memory  internal/handbook  internal/meter  internal/provider  internal/native
internal/org                     domain types: Bot, Team, Grant, Conversation, Message, Task, Run
internal/store                   SQLite, migrations, repositories; knows org types, nothing above
proto/hearth/v1                  the contract; generated Go under internal/rpc/gen, Swift under App/Packages/HearthKit
```

Rules:

- `org` holds plain types and pure functions (grant resolution, hierarchy checks, budget arithmetic). It imports nothing from hearth.
- `store` is the only package that imports the SQLite driver. Repositories return `org` types. One writer connection, WAL, append-only migrations.
- `agent` depends on `llm` and on three seams it is handed: a stream function, a tool executor, an approval gate. It never imports `store`, `tools` or `rpc`. This is the shape it already has in shhh and is what makes it liftable.
- `orchestrate` is the only package that creates runs, moves task state, and decides who runs next. It owns the event bus that `rpc` streams to the app.
- `tools` holds the registry and the core-side tools (files, web, assign, ask_owner, remember). `native` holds the proxies that forward calendar, contacts, shortcuts and on-device-model calls to the app over the callback channel; to `tools` they look like any other tool.
- `provider` wraps `llm`: tier mapping, credential injection, the ChatGPT account, model catalog queries. Nothing else imports `llm` directly except `agent`.
- `rpc` is translation only. A handler that contains an `if` about the domain is a defect.

### 2. Package map (first cut)

| Package | Responsibility | Origin |
|---|---|---|
| `internal/org` | Domain types, grant resolution, hierarchy rules, budget math | new |
| `internal/store` | SQLite repositories, migrations, export/import | shhh pattern |
| `internal/agent` | Turn loop, tool rounds, steering queue, repeat detection, retry, compaction | lifted from shhh `internal/agent` core + `retry.go` + `compact.go` |
| `internal/orchestrate` | Task state machine, run scheduling, fan-out and resumption, budgets, schedules, event bus | new |
| `internal/tools` | `Definition{Tool, Execute, Describe}` registry; files, web, assign, ask_owner, remember | shhh shape, new contents |
| `internal/tools/describe` | Plain-English effect strings for approval cards | shhh `describe` precedent |
| `internal/native` | Tool proxies to the app's NativeToolService | new |
| `internal/provider` | llm wiring, tiers, credentials, ChatGPT account, catalog | new, thin |
| `internal/meter` | Usage ledger, pricing, budgets | lifted from shhh |
| `internal/memory` | Bot notes: bounded recall, consolidation, pins | new (D13; shhh's memory package stays in shhh) |
| `internal/handbook` | Pages, scoping, context meter | new |
| `internal/secret` | Scrub tool results | lifted from shhh |
| `internal/skill` | Role templates as SKILL.md bundles | lifted from shhh |
| `internal/rpc` | Connect handlers, event stream, version handshake | new |
| `internal/framing` | The fixed instructions wrapped around job descriptions (Chief of Staff, lead, member) as embedded text with tests that they render | new |
| `cmd/hearthd` | Assembly, `--repl` harness for driving the core without the app | new |

Each package states its contract the Go way: a `doc.go` whose package comment names the responsibility, the invariants the type system does not hold, and the seams it is handed. Behaviour that spans packages (assignment, approvals, budgets, schedules) is already written down in `model.md#orchestration`; code cites those headings. No per-package spec files and no approval states: shhh's convention, which has held up, rather than overwatch's, which became paperwork.

### 3. Lifting from shhh (D8: extract first)

The packages listed as "lifted" are extracted from shhh into a shared Go module beside llm (working name `rfizzle/agentcore`; final name open) before hearth's agent loop work starts, with coding-specific imports (classifier, mode, patterns, tools) left behind in shhh. shhh is repointed at the module under its own gates and the module is tagged like llm (two-phase tag, never moved). hearth consumes the tag. This is M0 item 0.6 and it gates M1.5; it was chosen over copy-then-extract on 2026-10-19 so the two apps never diverge.

### 4. Gates

Every story passes, in this order, before it lands:

```
make fmt-check vet lint test docs-check     # shhh's set, same golangci config shape
make astimate                                # astimate check --base <landing head>; new in hearth: wired into make ci
make proto-check                             # buf lint + breaking against the last tag; generated code is committed and must match
```

astimate: ship `astimate.yaml` as the embedded default from the astimate repo plus hearth's exemptions section. Capacity ceilings are absolute, so the package map above is designed with them in mind: `orchestrate` and `store` are the two packages most likely to approach a ceiling and are split by responsibility early (`orchestrate/schedule`, `store/migrate`) rather than later.

Test tiers follow shhh's `testing` skill: hermetic by default (fake stream function, `t.TempDir`, in-memory SQLite file), `contract` build tag for anything that opens a real socket or talks to a real provider, `integration` for the full app-plus-helper launch on a macOS runner.

### 5. The core without the app

`hearthd --repl` drives the core from a terminal: create an org from a template file, send a message, watch the event stream, decide approvals. It exists so that orchestration, budgets and framing can be developed and evaluated without Xcode, and so the twenty-scenario evaluation (roadmap M5.7) runs in CI against the core alone. Native tools are stubbed with canned answers in this mode.

## Swift app

### 1. The thin-client rule, stated so it can be reviewed

1. The app holds **no business state**. Every fact about the org comes from the core over RPC; every change is an RPC. A view model may cache, sort, filter and animate; it may not decide.
2. A test for "what happens when the budget is exceeded" lives in Go. A test for "the paused card shows the resume affordance when the task is paused" lives in Swift, against fixture state.
3. The only Swift code that *does* things is in `HearthNative`: EventKit, Contacts, Shortcuts, Keychain, notifications, the on-device model. It is exposed to the core as a tool server and is otherwise inert.
4. If a feature seems to need logic in Swift, the contract is missing an RPC or an event. Fix the proto.

### 2. Package layout

Local Swift packages under `App/Packages/`, each with its own tests. The app target is a composition root and nothing else. Package boundaries are the Swift analogue of Go package boundaries: imports point down only, and a package that needs something from a higher layer is a design error.

```
App/
  Hearth/                     app target: scenes, windows, menus, helper lifecycle, DI wiring
  HearthGallery/              second app target: every component and composition in every state
  Packages/
    HearthKit/                generated Connect client + models, HearthClient protocol,
                              LiveClient (socket) and PreviewClient (fixtures). No SwiftUI import.
    HearthDesign/             tokens (asset catalog + Theme), primitives: domain-free atomic views
    HearthUI/                 components and compositions that know the domain models
    HearthFeatures/           one folder per feature: an @Observable store + screens
    HearthNative/             EventKit, Contacts, Shortcuts, Keychain, UNUserNotification,
                              Foundation Models adapters; the NativeToolService server
```

Dependency direction: `Hearth` → `HearthFeatures` → `HearthUI` → `HearthDesign`, with `HearthKit` importable by everything above `HearthDesign` and `HearthNative` importable only by `Hearth`. `HearthDesign` imports nothing of hearth's.

### 3. Unit taxonomy (the overwatch vocabulary, in SwiftUI terms)

| Unit | Definition | Lives in | Described by |
|---|---|---|---|
| **Token** | A semantic colour, type style, spacing, radius, shadow or motion value named by role, with light and dark variants | `HearthDesign` asset catalog + `Theme` | `Theme.swift` header comment + `tokens-check` |
| **Primitive** | A domain-free atomic view backed by one system control or a plain shape: `Avatar`, `StatusDot`, `Pill`, `CardSurface`, `DisclosureRow` | `HearthDesign` | header comment + `PreviewState` |
| **Component** | An atomic, domain-aware view of one model with no state of its own beyond presentation: `TaskStateBadge`, `ToolRow`, `CostLabel`, `DecisionBar` | `HearthUI` | header comment + `PreviewState` |
| **Composition** | An assembly of two or more existing units with its own local state and interaction flows, accessibility delegated to children: `TaskCard`, `MessageRow`, `ApprovalCard`, `SidebarOutline`, `Inspector` | `HearthUI` | header comment + `PreviewState` |
| **Feature** | A screen or flow: a store plus the compositions it drives: Conversation, Inbox, Onboarding, Settings, Usage, Handbook, Hire | `HearthFeatures` | the plan from plan-task, with a unit inventory |

"Backed by a system control" replaces overwatch's "backed by a Bits UI primitive": a primitive names the SwiftUI or AppKit control it wraps (`Button`, `List`, `NSTextView`) and inherits that control's keyboard and VoiceOver behaviour rather than reimplementing it. Custom drawing is allowed only where no control fits, and the header comment says why.

### 4. State pattern

- **Observation, not TCA.** Each feature has one `@Observable` store that subscribes to the core's event stream for its slice, exposes view state, and turns user intents into RPC calls. The Composable Architecture was considered and rejected: its reducer layer would duplicate the core, which is the real reducer, and it imposes a learning curve on every contributor.
- **Views are functions of store state.** No `@State` for anything that is domain state. `@State` is for presentation: disclosure open, hover, scroll anchor, draft text.
- **One client protocol.** `HearthClient` has a `LiveClient` and a `PreviewClient` driven by fixture files in `HearthKit/Fixtures`. Every preview, gallery page and snapshot test uses the same fixtures, so a state seen in a preview is a state the test covers.
- **Streaming** arrives as events on the subscription; stores apply deltas to their cached message list. Reconnect replays from the last sequence number the store saw.
- **Dependency injection** is manual, at the app target: stores receive the client in their initialiser; views receive stores through the environment.

### 5. Describing a unit without a bureaucracy

The overwatch workflow (a spec file per unit, draft/approved status, human sign-off, build refuses on draft, a revision for any deviation) was judged too heavy in practice. Hearth keeps the taxonomy and drops the ceremony. What stops the app from needing a rewrite is structure the tools enforce, not documents people approve:

| What keeps the shape | Enforced by |
|---|---|
| Package dependency direction | SwiftPM: a package cannot import what it does not declare |
| Thin-client rule | Review checklist; stores that import `HearthNative` or branch on domain rules are rejected |
| Tokens by role, no raw colours or fonts | A SwiftLint custom rule bans `Color(red:`, `.blue`-style literals and `Font.system(size:` outside `HearthDesign` |
| Every unit has every state | The gallery target fails to build if a unit in `HearthUI` or `HearthDesign` has no `PreviewState` |
| Visual regressions | Snapshot goldens compared in CI |
| Placement in the taxonomy | Review; the only question is "primitive, component, composition or feature?", answered by the folder it lands in |

The description of a unit lives in the unit, so it cannot drift away from it:

- **Header comment** at the top of the file: purpose in one or two sentences, the system control it is backed by (primitives), the children it composes (compositions), and anything non-obvious about accessibility or layout. Written before the body, edited in the same change as the body. No status field.
- **`PreviewState`** enum: the list of states, kept current by the compiler.
- **`#Preview` per state** using `PreviewClient` fixtures: the stories.
- Decisions that span units (why the task card has one shape everywhere, what the three columns mean, the vocabulary) live in `docs/model.md` (the task card rule, the vocabulary) and are cited from code with `// See docs/model.md#...`. Nothing in `docs/research/` is ever cited from code.

For the handful of shared units that everything else reuses (tokens, the primitives, and `TaskCard`, `MessageRow`, `ApprovalCard`), it is worth agreeing the states and the API in conversation before building, because a change there touches every screen. That agreement is a short exchange and a header comment, not a document with a status.

A new screen gets a one-page plan from the existing plan-task workflow with one extra section: the unit inventory (reuse / enhance / new primitive / new component / new composition) and the build order. That plan is the brief; there is no separate brief format.

### 6. Previews, gallery and goldens

- Every component and composition declares a `PreviewState` enum and a `#Preview` per state. This is the story and the state list in one place.
- `HearthGallery` is an app target that walks every `PreviewState` and renders it in light and dark, at default and the largest accessibility text size. It is what the owner looks at when a unit changes, and it is what SwiftUI-Kit does for system controls.
- `swift-snapshot-testing` records a PNG per (unit, state, appearance) under `App/Goldens/`. CI compares. A golden changes only in a change that says why, the same rule as shhh's goldens.
- Store tests run against `PreviewClient` fixtures and assert view state, not pixels.
- UI tests (XCUITest) cover the golden path of each screen and run in the integration tier.

### 7. AppKit escape hatches

Allowed, by name, in `HearthDesign` or `HearthUI` only, each with a header comment that says why SwiftUI was insufficient: the composer (`NSTextView` via `NSViewRepresentable` for attachments, mentions and paste), and if needed the transcript (`NSTableView`-backed list for cross-message selection). Any other `NSViewRepresentable` needs a reason in review. `swiftui-introspect` is not used; reaching into SwiftUI internals is the kind of coupling that breaks on every OS release.

### 8. Third-party packages

Minimal by policy; every dependency is a maintenance cost on a long-lived app. Of the fifteen in the 2026 Swift Kit article, four apply here; the rest are iOS indie-app concerns (payments, analytics, remote images, animation, backend SDKs) that hearth does not have. The Medium article could not be read (bot wall).

| Package | Use | Decision |
|---|---|---|
| `connectrpc/connect-swift` + `apple/swift-protobuf` | Generated client for the core | yes |
| `pointfreeco/swift-snapshot-testing` | Goldens | yes |
| `sparkle-project/Sparkle` | Updates | yes (M5) |
| `gonzalezreal/swift-markdown-ui` | Markdown rendering with tables and code blocks | yes, unless `AttributedString(markdown:)` plus a small table renderer proves enough in M1 |
| `sindresorhus/KeyboardShortcuts` | User-configurable global hotkey | yes, M4 only |
| `nalexn/ViewInspector` | View-tree unit tests | no by default; store tests plus snapshots cover it, and it tracks SwiftUI internals |
| `siteline/swiftui-introspect` | SwiftUI internals access | no (§7) |
| `kishikawakatsumi/KeychainAccess` | Keychain wrapper | no; the Security framework calls are a hundred lines in `HearthNative` and the surface is tiny |
| `apple/swift-log` | Logging | no; `os.Logger` directly |
| Alamofire, Kingfisher, Nuke, Lottie, RevenueCat, PostHog, TelemetryDeck, Supabase, MacPaw OpenAI | networking, images, animation, payments, analytics, backend, provider client | no; not applicable to a local thin client with no telemetry |

### 9. Quality gates for Swift

```
make swift-format-check     # swift-format (Apple's), config committed
make swift-lint             # SwiftLint: the size and complexity rules as the Swift ceiling
                            #   (cyclomatic_complexity, function_body_length, type_body_length,
                            #   file_length, nesting) plus the custom token rule
make swift-test             # xcodebuild test: package unit tests + snapshot comparison
make tokens-check           # contrast for every light/dark pairing against WCAG 2.2 AA (a Go test)
make gallery-build          # the gallery target compiles: every unit has a PreviewState
```

The intended end state is astimate judging Swift packages with the same rules as Go. astimate's extractor interface is language-agnostic and its TypeScript extractor is a pure-Go tree-sitter runtime, so a Swift extractor with `tree-sitter-swift` is a bounded addition to the astimate repo. Until it exists, SwiftLint's size and complexity rules are the ceiling.

### 10. The first units to build (before any screen)

In build order, because everything above them reuses them: tokens; `Avatar`, `StatusDot`, `Pill`, `CardSurface`, `DisclosureRow`; `TaskStateBadge`, `ToolRow`, `CostLabel`, `DecisionBar`; `TaskCard`, `MessageRow`, `ApprovalCard`, `QuestionCard`, `DeliverableCard`; then the Conversation screen. `TaskCard` is used in five places and is the composition to get right first.

## Skills and agents

### 1. Copy from existing repos

| Skill | From | Edit |
|---|---|---|
| `documentation` | shhh | Point the "where a fact belongs" table at hearth's `docs/` files; keep the citation convention and `make docs-check`. |
| `testing` | shhh | Swap in hearth's fixtures: fake stream function, in-memory store, stubbed native tools under `hearthd --repl`. |
| `sound-patterns` | shhh | None. |
| `go-conformance-suite` | global | None. Applies to tool executors, the credential store and native tools, each with a live and a fake implementation. |
| `golang-code-style`, `golang-naming`, `golang-error-handling`, `golang-structs-interfaces`, `golang-concurrency`, `golang-context`, `golang-testing` | go-starter | None expected. Add others only when a story needs them. |

Not copied: `backend-spec`, `component-spec`, `composition-spec`, `component-build`, `composition-build`, `feature-decompose`, `project-conventions` and the spec validators. Their useful content (the taxonomy, build order, reuse-first, tokens by role) is folded into the two new skills below.

### 2. Write new

| Skill | Purpose | Triggers |
|---|---|---|
| `swiftui-structure` | The whole of `architecture.md#swift-app` as working guidance: thin-client rule, package layout and import direction, the unit taxonomy and where a new view goes, Observation stores, `PreviewState` and gallery, goldens, tokens by role, the AppKit escape hatch rule, the accessibility checklist (VoiceOver label/value/traits, full keyboard access, Reduce Motion, Increase Contrast, Dynamic Type, 24 pt targets). | Any change under `App/`. |
| `go-structure` | The four layers and import rules from `architecture.md#go-core`, the `doc.go` contract convention, which packages are lifted from shhh and how a fix is ported back, astimate as the ceiling. | Any change under `internal/` or `cmd/`. |
| `proto-contract` | How to change `proto/hearth/v1`: edit, `buf lint`, `buf breaking` against the last tag, regenerate both sides, commit the generated code. The thin-client rule in reverse: if Swift needs logic, add an RPC or event here. | Any change under `proto/`, or a Swift change that wants domain logic. |
| `core-repl` | Driving `hearthd --repl` to reproduce a bug, run a scenario from the evaluation set, or demonstrate orchestration without the app. | Debugging orchestration, budgets, framing; running evals. |
| `mac-release` | Runbook: signing inside-out, `notarytool`, `stapler`, `vtool` checks, Sparkle appcast and keys, the release checklist. | Tagging a release or touching the build pipeline. |

Two skills carry the structure; three carry mechanics nobody should have to rediscover. If a sixth is proposed, the test is whether a tool or a reviewer acts on it.

### 3. Agents

The global set is enough: `planning` for a screen's plan (with the unit inventory and build order as a section), `implementation` for the work, `review` and `diff-reviewer` with the thin-client rule and taxonomy placement on their checklist, `verification` for the gates, `architect` for the orchestration state machine and for the security review of the tool gate before M1 lands. No spec-drafter and no spec-enforcer.

### 4. Gates that replace approvals

Everything that overwatch gated by a human approving a markdown file, hearth gates mechanically or not at all:

| Was | Now |
|---|---|
| Spec approved before build | A short conversation for the shared units only (tokens, primitives, `TaskCard`, `MessageRow`, `ApprovalCard`); header comment written first for everything |
| All spec states implemented | Gallery build fails on a missing `PreviewState`; goldens cover each state |
| Props widening needs a revision | Review |
| Semantic tokens only | SwiftLint custom rule |
| Contrast validated | `make tokens-check` |
| Spec/code drift | Nothing to drift: the description is in the file |

### 5. Bootstrap order (roadmap 0.5)

1. `.agents/skills/` with the copied skills and the symlink convention.
2. `AGENTS.md` in shhh's section order: what it is, scope and restraint, documentation rules, commands table, testing, before a push, version control, architecture (the package map and package layout above), invariants (thin-client rule, grants checked in the core, no credentials in context), gotchas.
3. The two structure skills, then the three mechanics skills as their subject arrives (proto with 0.1, repl with 0.6, release with M5).
4. Then the first unit: `Theme.swift` and `tokens-check`.
