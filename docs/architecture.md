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
   ┌──────────────────────────┐loopback TCP, token┌──────────────────────────────┐
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

**Why Connect-RPC over a Unix socket.** One proto file is the contract; generated Swift and Go clients; server-streaming maps directly onto token streaming; it works over a socket with no ports and no TLS. JSON-RPC (shhh's `internal/rpc`) is the fallback if the Swift Connect toolchain proves heavy; the proto still defines the messages. (D3, decided 2026-10-19; D16 settled the transport: loopback TCP with a per-launch token)

### 2. Responsibilities

| Concern | Lives in | Notes |
|---|---|---|
| Org, bots, teams (planned), conversations, tasks (planned), runs, usage, handbook, notes | core | SQLite is the source of truth. The app holds view models only. |
| Provider calls, streaming, retries | core | llm for calls; the retry loop is lifted from shhh (`internal/agent/retry.go` precedent), since llm only classifies failures. |
| Agent loop | core | Lifted from shhh `internal/agent` (passive state machine with StreamFunc / ToolExecutor / ApprovalGate seams), stripped of the coding-specific classifier, modes and tools. |
| Orchestration (assign, waits, budgets, schedules (planned)) | core | New; `model.md#orchestration` is its spec. |
| Tool registry | core | shhh's `Definition{Tool, Execute, Receipt}` shape. Two tiers only: read and act; act tools consult the approval gate. |
| Core-side tools | core | files.read/write/delete (within granted paths handed over by the app as resolved paths), web.fetch, web.search (offered only while a search key is stored), assign, ask_owner, remember. |
| Document reading and rendering | core | pure Go; the app only for text recognition through the native service |
| Shaped deliverables and measures | core renders, app shows | One spec, two renderers; measures by id through `MeasureService`. |
| Native tools (planned) | app, invoked by core | calendar.*, reminders.*, contacts.read, shortcuts.run, notifications. The core calls back into the app over the same connection (bidirectional stream, or the app registers as a tool server on a second socket). The app owns the TCC prompts and the EventKit/Contacts/Shortcuts code. |
| Credentials | app (Keychain) → core (memory) | The app reads its Keychain records and pushes them with `SyncCredentials` on connect; the core holds them in memory and writes none to disk. A changed record reaches the app as a `credential_changed` event (name only) and an `ExportCredential` call. The core hands back the records its own files held that the app lacked; a later sync that holds every one of those names deletes `keys/` and `chatgpt/`. A core started by hand with no app keeps owner-only files in those folders (D29). |
| ChatGPT plan tokens | core, in memory, over llm's `credstore.Store` | llm's `chatgpt.Open` takes a `Store`; the core gives it `provider.MemoryKeys`, which keeps the record under `chatgpt` and the host id under `chatgpt-host` and fires the change on every save, token refresh included. The app's Keychain is the durable copy. The file store serves a hand-run core only (D29). |
| Security-scoped bookmarks | app | Folder grants outside the Workspace are picked with NSOpenPanel and stored as bookmarks by the app; the app resolves them and tells the core the real path and the grant. Relevant only if a sandboxed build ships. |
| Workspace folder watching | app (FSEvents) → core | The app notifies the core of changes; the core indexes names and sizes only. |
| On-device model | app | Foundation Models for titles, rolling summaries of short histories, and routing hints. Called by the core through the callback channel; the app answers "unavailable" and the core falls back to the Fast tier. |
| Notifications, Inbox badges, menu bar | app | Driven by a core event stream (`Subscribe`). |
| App Intents, Spotlight, Share, Services | app | Thin wrappers over core RPCs. |

### 3. The contract (sketch)

One `hearth.v1` proto package, fourteen files. Built today:

```
HearthService     Ping, StreamHello (the spike)
OrgService        GetOrg, UpdateOrg
BotService        ListTemplates, CreateBot, ListBots, GetBot, DraftBot, UpdateBot, ListGrants, SetGrants, ListNotes, EditNote, DeleteNote, PinNote
HandbookService   ListPages, PutPage, DeletePage (the pages every bot reads, with a context meter)
ConversationService  ListConversations, GetConversation (with the latest run), ListMessages (paged), Send(returns run), Cancel, Retry
ProjectService    ListProjects, CreateProject (or confirm a proposal), UpdateProject, MarkProjectDone, ArchiveProject
InboxService      ListInbox, Decide(approval), Answer(question, or a widget), Acknowledge
UsageService      Summary(by bot/month), SetBudgets, Resume
ProviderService   ListAccounts, SetAPIKey, ClearCredential, StartChatGPTLogin (returns the page the app opens), ListKnownServices, ListServiceModels (before or after a key is saved), Tiers, SetTiers, SyncCredentials, ExportCredential
IntegrationService  ListIntegrations, SetIntegrationKey, ClearIntegration (web search backends)
MeasureService    ListMeasures, ReadMeasures (a closed registry of measure ids; the app re-reads on events it already receives)
DeliverableService  ListDeliverableTemplates, SaveDeliverableTemplate, DeleteDeliverableTemplate (the shipped five and the owner's, in the Templates folder)
EventService      Subscribe(stream): message deltas, run state, inbox changes, project changes, usage ticks, note, handbook, credential (name only) and tier changes
```

Planned, not in the proto yet:

```
OrgService        ExportOrg, ImportOrg
BotService        Delete
TeamService       List, Create, Update, Delete
TaskService       List, Get, Assign, Cancel, Retry, ListSchedules, UpsertSchedule
NativeService (planned; served by the app, called by the core): Ping, ExtractText, Calendar*, Reminders*, Contacts*, RunShortcut, OnDeviceGenerate, StoreCredential/LoadCredential, Notify
```

Streaming: `Send` returns immediately; the token stream arrives on `Subscribe` as `MessageDelta{conversation, message, text}` so the UI has one place to consume events and reconnects are simple. A heartbeat keeps a quiet subscription under the client's idle timeout, and the app reconnects from the last sequence it saw.

Lifecycle: the app launches `hearthd` with a per-launch token, the core binds a loopback port and reports it, and exits when the pipe the app holds closes. The app restarts it on crash and shows a quiet banner. Version handshake on connect; mismatch is a bug, not a user-facing state, because they ship together.

### 4. Data

- **SQLite** via `modernc.org/sqlite`, single writer connection, WAL, append-only migrations with a `schema_version` table (shhh pattern). Location: `~/Library/Application Support/Hearth/hearth.db`.
- Tables (first cut): `org`, `bots`, `teams`, `team_members`, `grants`, `conversations`, `messages`, `attachments`, `tasks`, `runs`, `usage`, `handbook_pages`, `bot_notes`, `schedules`, `projects`, `inbox_items`, `providers`, `tiers`, `settings`.
- `messages` follows shhh's `chat_messages` shape (session, seq, role, content, tool_calls JSON, tool_call_id, attachments) plus `author_bot_id`, `kind`, `task_id`, `run_id`, `cost`.
- **Files** live in the Workspace folder; the database stores relative paths and content hashes. Deleting a bot or team never deletes files.
- **Export** is a zip: `org.json` (bots, teams, grants without bookmarks, tiers without credentials), `handbook/*.md`, `templates/*.json`. No conversations, notes, usage or credentials.
- **Backups**: the Workspace is the user's; the database is covered by Time Machine. No hearth-specific backup in v1.

### 5. Security model

- Bots are a *permission* boundary, not a process boundary. Every tool call is checked against the calling bot's grants in the core before dispatch, and act-tier tools go through the approval gate. This is the opposite of Grok Bot's shared-computer design and is a stated differentiator.
- Credentials never enter model context. Tool results are scrubbed for the known key shapes before they are appended (planned: lift shhh's `secret` scrub).
- Web fetch has no cookies, follows redirects with a cap, strips scripts, limits size, and blocks link-local and private ranges.
- Prompt injection from fetched pages and files is treated as untrusted data: the framing says so, and act-tier tools require approval regardless of what a page says. There is no "always allow" for `files.delete` or any Shortcut that the owner did not individually grant.
- The core listens on loopback only and checks the launch token on every request (D16).
- No telemetry. Crash reports only with an explicit opt-in, via the macOS mechanism.

### 6. Build, sign, ship

- Repo: single repo. Go module at the root (`cmd/hearthd`, `internal/...`, `proto/`), SwiftPM package under `App/`. The Makefile builds the Go helper for arm64 (and amd64 if Intel is supported, then `lipo`), and `make app-build` assembles the bundle and copies it into `Contents/Helpers`. (D4, decided 2026-10-19)
- Signing inside-out: helper with hardened runtime and timestamp, then the app; `notarytool` + `stapler`. Pure Go needs no JIT entitlements. Check `LC_BUILD_VERSION` with `vtool` in CI.
- **Developer ID, notarised, Sparkle 2 updates** first. A Mac App Store build is a possible later SKU that would lose Shortcuts execution, Mail automation and wide file access, and would need the helper signed with `app-sandbox` + `inherit`. (D5, decided 2026-10-19)
- Licence MIT or Apache-2.0 to keep the App Store option open and to satisfy OpenAI's open-source sign-in programme. (D6, decided 2026-10-19)
- Minimum macOS 26 (Foundation Models, Liquid Glass, current App Intents). Apple Silicon only unless there is a reason to pay for Intel. (D7, decided 2026-10-19)
- CI: GitHub Actions on macOS runners: `make ci` for Go (fmt-check, vet, lint, test, docs-check, skills-check, astimate ceilings), `make ci-swift` for Swift, a notarised build on tags.

### 7. What is lifted from shhh and how

The package-by-package verdict on what to lift from shhh, as assessed when the architecture was first written, is frozen in [research/2026-10-19-shhh-lift.md](research/2026-10-19-shhh-lift.md).

### 8. Alternative considered: pure Swift

A single-language app using Swift for providers (own OpenAI/Anthropic/Gemini clients, own OAuth), SwiftData for storage and Swift concurrency for the loop. It would be simpler to sign, easier to put on the App Store, and more idiomatic. It was set aside because it duplicates llm and the shhh agent core, splits provider behaviour across the two apps, and the owner's intent is for hearth to be the third consumer of the shared Go code. D1 was decided for the Go core on 2026-10-19 (`decisions.md`); the alternative is recorded here for the reasoning.

## Go core

### 1. Layering

Four layers, dependencies point down only. astimate's cross-package duplication rule and golangci's `depguard` enforce the edges.

```
cmd/hearthd                      assembly only: flags, wiring, signal handling
internal/rpc                     Connect handlers; translate proto <-> domain; no logic
internal/orchestrate             tasks, runs, scheduler, budgets, waits (model.md#orchestration)
internal/assembly                a run's context within byte caps, dropped files, the summary and the notes' tidy
internal/agent                   one bot's turn: the loop, tool rounds, retry, compaction (lifted from shhh)
internal/tools  internal/handbook  internal/meter  internal/provider  internal/integration  (internal/document, internal/deliverable, internal/measure, internal/render/chart, internal/render/html, internal/render/paper and internal/render/fonts exist; the rest of render and native are planned)
internal/org                     domain types: Bot, Team, Grant, Conversation, Message, Task, Run
internal/org/widget              the closed catalogue of controls a bot may ask with; stdlib only
internal/org/intro               a bot's introduction card and the rule that assembles one; stdlib only
internal/project                 a project's types and the status rule derived from its tasks; imports org and the stdlib only
internal/workspace               the Hearth folder's layout and path containment; imports org only
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
| `internal/org/widget` | The seven controls a bot may ask with, their limits, and the fixed words an answer reaches the bot in (decisions.md D28) | new |
| `internal/org/intro` | A bot's introduction (purpose, three how lines, three prompts), the rule that assembles one for a bot hired from a description, and the check a shipped one passes; app-rendered, never in a run | new |
| `internal/org/inbox` | What waits for the owner: Inbox items, the four decisions on an approval, the allowances a decision leaves, and the rule that turns a decision into grants | new |
| `internal/org/memory` | Bot notes and handbook pages, their caps, and the pure rules for which pages a bot reads and how full the handbook is | new |
| `internal/org/usage` | Usage records, the month's totals, budgets, and the rule that says how near a budget the spending is | new |
| `internal/project` | Project types and the derived status rule | new |
| `internal/draft` | Turns a few words from the owner into a proposed bot with one model call outside any conversation | new |
| `internal/workspace` | The Hearth folder's layout and path containment | new |
| `internal/store` | SQLite repositories, migrations, export/import | shhh pattern |
| `internal/store/migrate` | The ordered schema steps and the runner that applies the ones a file has not seen | new |
| `internal/store/books` | What the bots remember, read and cost: notes, handbook pages, usage, budgets, tiers; the store embeds it | new |
| `internal/store/sqlrow` | The row mechanics the store's repositories share: ids, time columns, one-row updates, transactions | new |
| `internal/agent` | Turn loop, tool rounds, steering queue, repeat detection, retry, compaction | lifted from shhh `internal/agent` core + `retry.go` + `compact.go` |
| `internal/orchestrate` | Task state machine, run scheduling, fan-out and resumption, budgets, schedules, event bus | new |
| `internal/orchestrate/after` | The work after a run that ended done: tidying the bot's notes and folding the conversation's older text into its summary | new |
| `internal/orchestrate/bus` | The event bus: a sequence number per event, a ring for resync, a heartbeat, and the bodies of the events the orchestrator publishes | new |
| `internal/orchestrate/park` | What a run that waits on the owner leaves behind: the thread message and the Inbox item, the wording of each decision, the grant for an always-allow | new |
| `internal/orchestrate/record` | What one run writes as it goes: its messages, steps and how it ended, stored and published; the agent's sink | new |
| `internal/orchestrate/toolset` | The tools one run is handed, built from the bot's grants | new |
| `internal/tools` | `Definition{Tool, Execute, Describe}` registry (`Describe` is planned); files, web, assign, ask_owner, remember | shhh shape, new contents |
| `internal/tools/ask` | `ask_owner`: a question in words or with one control, refused before it waits when the control is malformed | new |
| `internal/tools/propose` | `propose_project`: the Chief of Staff offers the owner a project as a card; the text is bounded before it is stored and nothing starts until the owner confirms | new |
| `internal/tools/memory` | `remember`: appends one note for the calling bot through functions the orchestrator hands it, and says when the notes are full | new |
| `internal/document` | Reads the common document kinds as text: kind by header, the caps, the notices and the picture-page rule; pure Go | new |
| `internal/render/docx` (planned) | Renders a Markdown file to Word from the same walk, for `files.render` | new |
| `internal/deliverable` | The spec of a shaped deliverable, the closed catalogue of ten widgets, the refusals in words, the canonical form; stdlib only | new |
| `internal/deliverable/template` | The five shipped templates and the owner's from `Templates/`; the positional check | new |
| `internal/measure` | The closed registry of measure ids and one read over the store, the meter and the project rule | new |
| `internal/render/chart` | One deterministic chart layout feeding the SVG and the PDF; the six hues | new |
| `internal/render/html` | A spec to a self-contained web page, no script, with a content-security-policy | new |
| `internal/render/paper` | A Markdown file to a PDF in the print style, reproducibly, for `files.render`; a spec's PDF with vector charts on the same page is planned | new |
| `internal/render/fonts` | The embedded faces a PDF is set in, Source Sans 3 and Source Code Pro, with their licence | new |
| `internal/tools/deliverables` (planned) | `deliverables.write`: validates before the gate, fills measures, writes the spec, the page and the PDF beside each other | new |
| `internal/rpc/deliverables` | The handlers of `MeasureService` and `DeliverableService` | new |
| `scripts/tokens` (planned) | Generates the chart hues and the Tailwind theme from the design tokens | new |
| `scripts/tailwind` (planned) | The pinned Tailwind standalone CLI's version and sums and the build that makes the page's stylesheet; run at build time only | new |
| `internal/native` (planned) | Proxies to the app's NativeService, ExtractText first | new |
| `internal/tools/files` | `files.read`, `files.write`, `files.render`, `files.delete` inside the folders a bot's grants name | new |
| `internal/tools/web` | `web.fetch` of public pages under the address policy, and `web.search` behind a seam, Brave first | new |
| `internal/provider` | llm wiring, tiers, credentials, ChatGPT account, catalog | new, thin |
| `internal/provider/gateway` | A service reached by address: the table of known services, the check on an address, the list of its models, the check that a completion is one | new |
| `internal/provider/models` | Which model a bot runs on: the tier mapping, one bot's pin, the models the accounts offer, the probe for a local runtime | new |
| `internal/keystore` | Named secrets: the Store interface, the file store, the memory store the app syncs into | new |
| `internal/integration` | The web-search key over the shared key store; builds the searcher a run is handed, none when no key is set (D22) | new, thin |
| `internal/meter` | Usage ledger over llm's prices, month summary, budget rails (D30) | hearth's own ledger: agentcore's is in memory, this one survives relaunch |
| `internal/handbook` | Pages by scope, the refusal of a page that overfills a reader's context, the "Handbook" block a run carries | new |
| `internal/skill` | Role templates as SKILL.md bundles | lifted from shhh |
| `internal/skill/builtin` | The shipped role templates as SKILL.md bundles | new |
| `internal/rpc` | Connect handlers, event stream, version handshake | new |
| `internal/rpc/staff` | The calls about the org and its people: the org, the bots with their grants, the conversations, the Inbox and the event stream, beside the spike | new |
| `internal/rpc/accounts` | The calls about the model accounts and the integrations, and the bus bodies their changes publish | new |
| `internal/rpc/books` | The calls about what the org keeps on its bots: notes, a bot's settings, the handbook, usage and budgets, the tiers and the models on offer | new |
| `internal/rpc/wire` | The translation the services share: the one place a domain error picks its code, the conversions more than one service needs, and the controls a question asks with | new |
| `internal/framing` | The fixed instructions wrapped around job descriptions (Chief of Staff, lead, member) as embedded text with tests that they render | new |
| `internal/assembly` | A run's context, built in a fixed order within byte caps (framing, handbook, notes, summary, window, the parked round, attachments); dropped files brought where the bot may read them; and the side runs after a run: the conversation's summary and tidying a bot's notes | new |
| `cmd/hearthd` | Assembly, `--repl` harness for driving the core without the app | new |

Planned, with no directory yet: `internal/tools/describe` (plain-English effect strings for approval cards), and a scrub of tool results for key shapes. Bot notes live in `org/memory`, `tools/memory` and `assembly`.

Each package states its contract the Go way: a `doc.go` whose package comment names the responsibility, the invariants the type system does not hold, and the seams it is handed. Behaviour that spans packages (assignment, approvals, budgets, schedules) is already written down in `model.md#orchestration`; code cites those headings. No per-package spec files and no approval states: shhh's convention, which has held up, rather than overwatch's, which became paperwork.

### 3. Lifting from shhh (D8: extract first)

The packages listed as "lifted" are extracted from shhh into a shared Go module beside llm (`github.com/rfizzle/agentcore`, D12) before hearth's agent loop work starts, with coding-specific imports (classifier, mode, patterns, tools) left behind in shhh. shhh is repointed at the module under its own gates and the module is tagged like llm (two-phase tag, never moved). hearth consumes the tag. The module stays v0 until both apps ship a milestone on one tag (D15); its gate checks the exported API against the previous tag, so a tag's message says what it broke. It was chosen over copy-then-extract on 2026-10-19 so the two apps never diverge.

### 4. Gates

Every change passes, in this order, before it lands:

```
make fmt-check vet lint test docs-check     # shhh's set, same golangci config shape
make astimate                                # astimate check --base <landing head>; new in hearth: wired into make ci
```

Once the proto exists, a check that the generated code is current joins them: buf lint and a breaking-change check against the last tag, with the committed generated code required to match.

astimate: ship `astimate.yaml` as the embedded default from the astimate repo plus hearth's exemptions section. Capacity ceilings are absolute, so the package map above is designed with them in mind: `orchestrate` and `store` are the two packages most likely to approach a ceiling and are split by responsibility early (`orchestrate/schedule`, `store/migrate`) rather than later.

Test tiers follow shhh's `testing` skill: hermetic by default (fake stream function, `t.TempDir`, in-memory SQLite file), `contract` build tag for anything that opens a real socket or talks to a real provider, `integration` for the full app-plus-helper launch on a macOS runner.

### 5. The core without the app

`hearthd --repl` drives the core from a terminal: create an org from a template file, send a message, watch the event stream, decide approvals. It exists so that orchestration, budgets and framing can be developed and evaluated without Xcode, and so the twenty-scenario evaluation runs in CI against the core alone. Native tools are stubbed with canned answers in this mode.

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
                              Foundation Models adapters; the NativeService server
```

Dependency direction: `Hearth` → `HearthFeatures` → `HearthUI` → `HearthDesign`, with `HearthKit` importable by everything above `HearthDesign` and `HearthNative` importable only by `Hearth`. `HearthDesign` imports nothing of hearth's.

### 3. Unit taxonomy (the overwatch vocabulary, in SwiftUI terms)

| Unit | Definition | Lives in | Described by |
|---|---|---|---|
| **Token** | A semantic colour, type style, spacing, radius, shadow or motion value named by role, with light and dark variants | `HearthDesign` asset catalog + `Theme` | `Theme.swift` header comment + `tokens-check` |
| **Primitive** | A domain-free atomic view backed by one system control or a plain shape: `Avatar`, `StatusDot`, `Pill`, `CardSurface`, `DisclosureRow` | `HearthDesign` | header comment + `PreviewState` |
| **Component** | An atomic, domain-aware view of one model with no state of its own beyond presentation: `TaskStateBadge`, `ToolRow`, `CostLabel`, `DecisionBar` | `HearthUI` | header comment + `PreviewState` |
| **Composition** | An assembly of two or more existing units with its own local state and interaction flows, accessibility delegated to children: `TaskCard`, `MessageRow`, `ApprovalCard`, `SidebarOutline`, `Inspector` | `HearthUI` | header comment + `PreviewState` |
| **Feature** | A screen or flow: a store plus the compositions it drives: Conversation, Inbox, Onboarding, Settings, Usage, Handbook, Hire | `HearthFeatures` | its plan, with a unit inventory |

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

The description of a unit lives in the unit, so it cannot drift away from it: a header comment, the list of its states kept current by the compiler, and a preview per state. Decisions that span units (why the task card has one shape everywhere, what the three columns mean, the vocabulary) live in `docs/model.md` and are cited from code.

The handful of shared units that everything else reuses are agreed before they are built, because a change there touches every screen. That agreement is a short exchange and a header comment, not a document with a status.

### 6. Previews, gallery and goldens

- The gallery is an app target that renders every unit in every state, in light and dark, at default and the largest accessibility text size. It is what the owner looks at when a unit changes, and it is what SwiftUI-Kit does for system controls.
- Snapshot goldens record each (unit, state, appearance) and CI compares them, so a state seen in the gallery is a state a test covers.
- Store tests run against fixtures and assert view state, not pixels.
- UI tests (XCUITest) cover the golden path of each screen and run in the integration tier.

### 7. AppKit escape hatches

AppKit is allowed, by name, in the design and UI packages only, where SwiftUI falls short: the composer (attachments, mentions and paste) and, if needed, the transcript (cross-message selection). `swiftui-introspect` is not used; reaching into SwiftUI internals is the kind of coupling that breaks on every OS release.

### 8. Third-party packages

Minimal by policy; every dependency is a maintenance cost on a long-lived app. Of the fifteen in the 2026 Swift Kit article, four apply here; the rest are iOS indie-app concerns (payments, analytics, remote images, animation, backend SDKs) that hearth does not have. The Medium article could not be read (bot wall).

| Package | Use | Decision |
|---|---|---|
| `connectrpc/connect-swift` + `apple/swift-protobuf` | Generated client for the core | yes |
| `pointfreeco/swift-snapshot-testing` | Goldens | yes |
| `sparkle-project/Sparkle` | Updates | yes |
| `gonzalezreal/swift-markdown-ui` | Markdown rendering with tables and code blocks | yes, adopted for deliverable text blocks only (D48), pinned to one release; the transcript keeps the inline-only string, and the image loader it brings is never handed an image |
| Charts (Apple's) | The chart framework for a shaped deliverable's native charts | yes; part of the SDK |
| `sindresorhus/KeyboardShortcuts` | User-configurable global hotkey | yes |
| `EmergeTools/Pow` | Expressive transitions and change effects | no; evaluated for the motion pass and not needed: every transition hearth uses is one SwiftUI modifier, and the effects it adds are the gratuitous kind (`docs/design.md#motion`) |
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
make swift-test             # swift test per package: unit tests + snapshot comparison
make tokens-check           # contrast for every light/dark pairing against WCAG 2.2 AA (a Go test)
make gallery-build          # the gallery target compiles: every unit has a PreviewState
```

The intended end state is astimate judging Swift packages with the same rules as Go. astimate's extractor interface is language-agnostic and its TypeScript extractor is a pure-Go tree-sitter runtime, so a Swift extractor with `tree-sitter-swift` is a bounded addition to the astimate repo. Until it exists, SwiftLint's size and complexity rules are the ceiling.
