# hearth · model

The things in the system, how they relate, and the rules that govern how work flows between the owner and bots. This is the behavioural contract the Go core implements and the app reflects; code cites these headings.

## Concepts

### The org

**Org** (`Org`). One per app. Has a name, an owner (the Mac user, always a human), a Handbook, a Workspace folder, usage budgets, and a set of providers signed in. Everything else hangs off it.

### People

**Bot** (`Bot`) — the user-facing word may be "assistant", "team member" or the bot's own name; "bot" is the code name.

| Field | Meaning | Who edits |
|---|---|---|
| name, avatar | Identity shown everywhere | Owner |
| role | Short title ("Research Lead") | Owner, from template or free text |
| job description | Plain-language paragraph(s): what they do, how they work, what good looks like. Rendered into the model's instructions together with hearth's fixed framing; the owner never sees the framing on the main path. | Owner |
| scope | `team(<id>)` or `org` | Owner |
| is lead | True for exactly one bot per team | Owner |
| access grants | The tools this bot may use and the folders it may touch (see Access) | Owner |
| quality tier | Fast / Balanced / Best, mapped to a concrete provider+model per tier in org settings; Advanced shows and overrides the model | Owner |
| notes | The bot's memory: a list of short plain-language facts it has learned, each with a date and a source link | Bot writes, owner edits/deletes |
| status | idle, working, paused (budget), disabled | System |

**Role template** (`RoleTemplate`). A starting point: role, job description, suggested access, suggested tier, suggested team. Shipped templates live in the app bundle; the owner's saved ones live in the org. A starter org is a set of templates plus a team layout.

**Team** (`Team`). Name, purpose (plain language), one lead, zero or more members, a Workspace subfolder, optional standing brief (what the team does when assigned something vague), a budget. A team channel is created with it.

**Chief of Staff** (`Coordinator`). The default org-level bot. It is a normal Bot with `scope = org` and the `assign` tool scoped to teams. It is the default recipient of anything typed in the main compose box. The owner may add other org-level bots (e.g. "Scheduler") but only coordinators may assign across teams.

Hierarchy, strictly:

```
Owner
 └─ Org-level bots (Chief of Staff, …)        may assign to: any team's lead, any org-level bot
     └─ Team lead                               may assign to: members of own team
         └─ Team member                         may assign to: nobody; may ask the owner
```

No peer-to-peer assignment. No upward assignment (a member cannot give the lead a task; it can *ask a question*, which goes up). This is deliberate, and it is the biggest difference from frameworks that allow arbitrary hand-offs.

### Conversations

**Conversation** (`Conversation`). Three kinds, same message model:

| Kind | Participants | Created by | Lives in sidebar as |
|---|---|---|---|
| **Direct message** (`dm`) | Owner + one bot | Owner | the bot's name |
| **Team channel** (`channel`) | Owner + lead + members | With the team | the team's name |
| **Task thread** (`task`) | Assigner + assignee (+ owner, always) | An assignment | nested under the conversation it was assigned from |

**Message** (`Message`). Author (owner or bot), body (rich text with attachments), kind (`text`, `tool_call`, `tool_result`, `assignment`, `result`, `question`, `approval_request`, `approval_decision`, `system`), timestamps, cost attribution. Tool calls and results render as collapsed cards by default.

**Attachment** (`Attachment`). A file reference (bookmark to a file in the Workspace or a granted folder) or an inline image/PDF. Dropped files are copied into the Workspace `Inbox` subfolder unless they already live in a granted folder.

### Work

**Task** (`Assignment`). The unit of delegation. Created when the owner or a bot assigns work.

| Field | Meaning |
|---|---|
| brief | Goal, context, expected deliverable and format, constraints. Written in full; never "see above". |
| assigner, assignee | Bot or owner → bot (or team, which resolves to its lead) |
| parent | The task this was spawned from, if any; depth is derived |
| thread | The task thread conversation |
| state | `queued` → `working` → (`waiting_owner` ⇄ `working`) → `done` / `failed` / `cancelled` / `paused_budget` |
| budget | Max cost, max steps, max wall time; inherited from parent and capped by team/org budgets |
| deliverables | Files written to the Workspace, listed on the task card |
| result | The assignee's final message, returned to the assigner's thread |
| schedule | Optional recurrence (see Recurring) |

**Run** (`Run`). One bot taking a turn: a model call loop with tool calls until the bot yields a message, a question, an assignment wait, or hits a limit. Runs are the unit of cost accounting and cancellation. A task is a sequence of runs by the assignee, interleaved with waits.

**Recurring task** (`Schedule`). A task brief plus a calendar rule (daily/weekly/monthly at a time, or "every N hours while I'm working"). Fires while the app or its menu bar extra is running; missed fires are listed, not auto-run, unless the owner says "catch up".

### Owner interaction

**Inbox** (`InboxItem`). Anything that blocks on the owner, in one list: approval requests, questions from bots, finished deliverables to acknowledge, budget pauses, missed schedules. Each item deep-links to its thread. Notifications mirror Inbox.

**Approval** (`Approval`). A request to use a tool whose grant is "ask me". Shows exactly what will happen in plain language (e.g. "Send an email to Dana Ortiz with subject 'Invoice March' via your 'Send Mail' shortcut"), with Allow once / Allow for this task / Always allow for this bot / Deny. Decisions are recorded in the thread.

**Question** (`Question`). A bot asking the owner something, with optional suggested answers. Pauses the task; the answer resumes it with the question and answer appended to the brief.

### Knowledge

**Handbook** (`HandbookPage`). Markdown pages the owner writes or imports (about me, my preferences, house style, project background). Every bot reads the pages tagged for its team or for everyone. Rendered as a simple page editor; no retrieval settings. Size is capped with a visible meter so the owner understands it all goes into context.

**Notes** (`BotNote`). Per-bot memory as above. A bot may add a note only via the `remember` tool, which shows in the thread; the owner can delete any note. Notes are read into every run by that bot.

**Workspace** (`Workspace`). A folder on disk. Layout:

```
Hearth/
  Handbook/            (exported copy of handbook pages, read-only mirror)
  Inbox/               (files the owner drops in without a home)
  <Team name>/         (deliverables, one subfolder per task when more than one file)
  Chief of Staff/      (org-level deliverables)
```

The app watches the folder; files the owner adds are visible to bots with the folder grant.

### Access

**Access grant** (`Grant`). A tuple of (subject: bot or team, capability, mode). Capabilities in v1:

| Capability | What it allows | Default for new bots |
|---|---|---|
| `files.read(<folder>)` | Read files under a folder (Workspace subfolders, or any folder the owner picks via the open panel, stored as a security-scoped bookmark) | own team's Workspace subfolder |
| `files.write(<folder>)` | Create/replace files | own team's Workspace subfolder |
| `files.delete(<folder>)` | Delete files | ask me |
| `web.fetch` | Fetch a URL (read-only, no cookies) | allowed |
| `web.search` | Web search via the provider's or a configured search tool | allowed |
| `calendar.read` / `calendar.write` | EventKit | read: ask me; write: ask me |
| `reminders.read` / `reminders.write` | EventKit | ask me |
| `contacts.read` | Contacts | ask me |
| `shortcuts.run(<name>)` | Run one named Shortcut with text input | not granted; owner picks per shortcut |
| `assign` | Delegate (scoped by hierarchy, not a grant the owner edits) | by role |
| `ask_owner` | Always available | always |
| `remember` | Always available | always |

Modes: `allowed`, `ask me` (approval each time, with "always allow" offered), `not granted`. Team grants apply to every member; a bot's grants add to its team's.

### Providers and models

**Provider account** (`ProviderAccount`). A signed-in account or stored key for one backend: ChatGPT plan (OAuth), Anthropic (key), Gemini (key), OpenRouter (key), Compatible endpoint (URL + optional key, how Ollama/LM Studio arrive), Apple on-device (no account). Stored in Keychain; the Go core receives credentials through the llm library's credential interface.

**Quality tier** (`Tier`). Org-level mapping of Fast/Balanced/Best to (provider, model). Defaults are set when the first provider is signed in. The owner can change the mapping in Settings; a bot's Advanced panel can pin a specific model.

### Usage

**Usage record** (`Usage`). Per run: provider, model, input/output/cached tokens, estimated cost from the llm catalog, attributed to bot, team, task and month. Displayed as "Usage" (Q7 decided against "Payroll"). Budgets are soft (warn) at 80% and hard (pause) at 100% per task, team and org per month.

### Vocabulary (user-facing)

| Code name | UI word | Notes |
|---|---|---|
| Org | your org / "Hearth" | Named during onboarding ("Coleton's Office"). |
| Bot | by name; collectively "your team" | The word "bot" appears nowhere in the UI. "Assistant" is avoided too. (Q7, decided 2026-10-19) |
| Coordinator | Chief of Staff | Fixed name for the default one; user can rename. |
| Team | Team | |
| Lead | Lead | Shown as a badge on the member row. |
| Assignment | Task | |
| Run | (not shown) | Appears only in Usage as "steps". |
| Grant | Access | "What Dana can see and do". |
| Handbook | Handbook | |
| BotNote | Notes | "What Dana remembers". |
| Usage | Usage | Cost in the user's currency, tokens under a disclosure. |
| Workspace | Hearth folder | Shown as a Finder-like path with a "Show in Finder" button everywhere files are mentioned. |


## Orchestration

### 1. Where a message goes

| Owner types in… | Addressed to | What happens |
|---|---|---|
| Main compose box (home) | Chief of Staff | A run starts for the Chief of Staff in the Chief of Staff DM. It answers directly or assigns. |
| A bot's DM | That bot | A run for that bot. A lead may assign to its members; a member answers itself. |
| A team channel | The team's lead | A run for the lead with the channel history as context; members are mentioned, not run, unless the lead assigns. |
| A task thread | The task's assignee | Appended to the task's context; the assignee's next run sees it. If the task is `waiting_owner`, it resumes. |

Owner messages are always allowed to interrupt: typing in a thread while a run is in progress queues the message and the run is told at its next tool boundary that new owner input exists.

### 2. A run

```
loop:
  context  = framing + job description + handbook (scoped) + notes + conversation window + task brief (if any) + pending results
  reply    = model(context, tools granted to this bot)
  for each tool call in reply:
     if tool needs approval   → create Approval, set task waiting_owner, END RUN (resumes on decision)
     if tool is ask_owner     → create Question,  set task waiting_owner, END RUN
     if tool is assign        → create child Task(s), set task waiting_children, END RUN
     else                     → execute, append result
  if reply has no tool calls  → emit message; if in a task thread and the message is marked final → task done
  check limits (steps, cost, wall time) → on breach: paused_budget + Inbox item, END RUN
```

Key properties:

- **Runs end at every wait.** A bot that is waiting on the owner or on a sub-task is not holding a model call open. Resumption builds fresh context including the new result. This keeps cost proportional to work and lets the owner intervene at every boundary.
- **Context window policy.** Conversation history is included newest-first up to a per-tier budget; older history is summarised by the on-device model (or the Fast tier) into a rolling summary stored on the conversation. The summary is visible under "Earlier in this conversation" so the owner can see what the bot remembers.
- **Tools are the only side effects.** A run cannot write files, send anything, or remember anything except through a tool call, and every tool call is a message in the thread.

### 3. Assignment (delegation)

The `assign` tool takes:

```
assign(to: team | bot, brief: {goal, context, deliverable, constraints}, budget?: {cost, steps, minutes}, parallel_group?: id)
```

Rules:

1. **Scope.** Org-level bots may assign to any team (resolved to its lead) or org-level bot. A lead may assign only to its own members. A member may not assign. The tool is simply absent for members, so the model cannot try.
2. **Depth.** Owner → Chief of Staff → lead → member is depth 3 and the maximum. A member cannot create a child, so the tree is bounded by construction.
3. **Full briefs.** The tool schema requires `goal`, `context` and `deliverable` as non-empty prose. The framing instructs the assigner that the assignee sees *nothing* else: not the parent conversation, not the owner's original message. If the assigner wants the assignee to see something, it must quote it or attach a file. This is the single most important rule from practitioner write-ups, and the UI shows the brief as the first message of the task thread so the owner can judge it.
4. **Fan-out.** One run may issue several `assign` calls; they execute in parallel (bounded by an org-wide concurrency limit, default 3) and the assigner resumes once *all* have reached a terminal state, receiving each result in order. A `parallel_group` lets the assigner say "these are independent"; without it the calls still run in parallel but the framing tells the assigner to prefer one assignment at a time unless the work is clearly separable.
5. **Results.** The assignee ends a task with a final message (`result`) plus deliverable files. The result is appended to the assigner's context as a tool result; files are referenced by path. The assigner is told to check the result against its brief before passing it up, and may re-assign with a follow-up brief at most twice per task (`max_retries = 2`).
6. **Budgets flow down.** A child's budget cannot exceed what remains of the parent's. Team and org monthly budgets cap everything. A breach pauses the subtree and raises one Inbox item at the top.
7. **No upward delegation, no sideways.** A member that needs something from another team says so in its result ("I could not get X; the Calendar team would need to…"); the lead or Chief of Staff decides. Questions to the owner (`ask_owner`) are the only upward channel and they pause the whole subtree.
8. **Cancellation.** Cancelling a task cancels its subtree. The owner can cancel from any card; a bot can cancel only its own children by re-assigning or finishing.

### 4. Termination guarantees

A task always reaches a terminal state because:

- every run has a step cap (default 25 tool calls) and a wall-time cap (default 10 minutes of active run time),
- every task has a cost cap derived from the parent and budgets,
- depth is bounded by construction,
- retries are capped,
- schedules do not overlap: a recurring task whose previous instance is still open skips and reports.

The org-wide concurrency limit also bounds the number of simultaneous model calls, which keeps rate limits and the usage meter predictable.

### 5. The Chief of Staff's routing policy

The coordinator is a normal bot with a strong job description shipped with the app. Behaviour it is instructed to follow (and which the UI reinforces by showing its reasoning as a short "Plan" card before it assigns):

1. Answer directly if the request needs no tools or needs only its own grants and is small.
2. Otherwise identify the one team whose purpose matches; assign with a full brief; wait; check; reply to the owner with the result and where the files are.
3. Fan out to several teams only when the owner's request has clearly separable parts. Say so in the Plan card.
4. Ask the owner before assigning if the request is ambiguous in a way that would change which team or what deliverable. One question, with suggested answers, not a questionnaire.
5. Never restate work that a team already did; link the deliverable.

A routing hint from the on-device model (team name from the request text) is passed to the coordinator as a suggestion, never enforced.

### 6. Leads

A lead's shipped framing adds: break the brief into member-sized assignments only when there is more than one member and the work is separable; otherwise do it yourself; always review a member's result against the brief before returning it; write the team's deliverable into the team folder with a sensible filename; keep the team channel informed with one short status message per milestone (not per step).

### 7. Memory writes

`remember(note)` appends a dated note to the calling bot's Notes with a link to the message. The framing says what is worth remembering: stable facts about the owner, preferences, decisions, recurring context. Not task progress. Notes are capped (default 200); when full, the bot is asked to consolidate (merge/delete) in a visible step. The owner can pin notes so they are never consolidated.

### 8. Approvals

Approval cards show the exact effect in plain words, generated by the tool's own describer (not by the model), so the owner is not trusting the bot's paraphrase. Decisions:

- **Allow once**: this call only.
- **Allow for this task**: the same capability for the rest of this task subtree.
- **Always allow for <bot>**: upgrades the grant to `allowed`, shown in the bot's Access sheet.
- **Deny**: the tool returns a denial to the model with the owner's optional reason; the bot continues.

Approval requests expire into the task's `waiting_owner` state indefinitely; schedules that hit an approval surface it in Inbox and in a notification.

### 9. Failure handling

- Provider errors: retried per llm's failure classification; on persistent failure the run ends with a `system` message in the thread and an Inbox item ("Research Lead could not reach OpenAI"), the task goes to `failed` with a Retry button.
- Tool errors: returned to the model once; a second identical failure ends the run with the error visible.
- App quit mid-run: runs are not resumable; on relaunch tasks that were `working` are set to `failed` with a Retry button that re-runs the last run from scratch. Deliverables already written stay.

### 10. What the owner sees

Each task has a card (in the thread it came from, in the team channel, and in Inbox when it needs them) with: assignee, state, elapsed time, cost so far, step count, deliverables, and a "Show work" disclosure that opens the task thread with every tool call and sub-task inline. The same card is used everywhere so the owner learns one shape.
