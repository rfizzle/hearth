# hearth · product

What hearth is, who it is for, and the strict line around what it will and will not do. Updated when the product changes, which is rarely. Decisions are in `decisions.md`.

This describes the product as designed. `AGENTS.md`'s map and `docs/architecture.md`'s package table mark what exists today.

## Vision

### One sentence

hearth is a native Mac app where you run a small organisation of AI employees: each one has a name, a job, a team and a boss, you talk to them the way you would talk to colleagues in Messages or Slack, and the work they do lands in a folder on your Mac.

### Who it is for

- One person at work: a founder or an employee at a small company, using hearth for their job. One Mac, one owner, one org. (Multi-user orgs are out of scope; see `product.md#scope-boundary`.)
- Less technical than a developer. They use Mail, Calendar, Notes, Shortcuts and ChatGPT. They have never seen a system prompt and should not have to.
- They want help with recurring business work: market research, marketing campaigns, finance and status reports, writing for customers, planning launches, keeping on top of a calendar and inbox. Every example, template and sample the app ships is drawn from that work, not from personal life.

### Why an org, and not "a chat with an AI"

The org metaphor is doing three jobs, in this order of importance:

1. **Scoping access.** "The Research team can read the web and my Research folder. Nobody can send email without asking me." People already understand roles and permissions in terms of jobs. This is the part of the metaphor that makes the app safe to give real access to.
2. **Routing without thinking.** You tell the Chief of Staff what you want; it goes to the right team and comes back done. You never have to pick the right prompt, model or tool.
3. **Repeatable work.** A team with a lead and a standing brief is a workflow you can run again ("Monday planning", "weekly status report") without writing automation.

It is explicitly **not** a simulation. Bots do not chat among themselves for colour. Every bot-to-bot message exists because of a task somebody assigned, and every one of them is visible to the owner in a thread. The published lessons on multi-agent systems are consistent: free-form agent conversation burns tokens and produces little, and users lose track of who is doing what. hearth's structure is the discipline that keeps delegation useful (see `model.md#orchestration`).

### Design principles

1. **Visible work.** Every delegation, tool call, approval and cost is a thing you can see in a thread or a card. No hidden agents.
2. **Deliverables are files.** Anything a bot produces that matters is a real file in the hearth Workspace folder in Finder, with Quick Look and Spotlight. Chat is the conversation, the folder is the output.
3. **Ask before acting outward.** Reading is cheap, acting is not. Sending, posting, deleting, buying and running anything outside the app require approval unless the owner has explicitly granted standing permission to that bot.
4. **Plain language everywhere.** Job descriptions, handbook pages and memory notes are sentences, not configuration. Advanced settings exist behind a disclosure, never on the main path.
5. **Native and quiet.** A Mac app that looks like it belongs: sidebar, inspector, menu bar extra, Shortcuts, Spotlight, notifications, Keychain. No web views for the core UI.
6. **Bring your own accounts.** The app is free and open source. Users sign in with the subscriptions they already have (ChatGPT plan via the open-source partner path, other providers by API key) and can see what each bot costs them.
7. **Bounded autonomy.** Depth, budget and time limits on every task, by default. A bot can always stop and ask.

### What success looks like (first public release)

- A new user installs the app, signs in with one provider, and within five minutes has a three-bot starter org and has had the Chief of Staff produce a real document in their Workspace folder.
- They can explain, in their own words, what each bot is allowed to do.
- They never saw the words "prompt", "token", "temperature" or "model" unless they opened Advanced.
- A week later they have a recurring task that runs while the app is open and they trust its output enough to act on it.

## Scope boundary

### hearth WILL

#### Product
- Run **one org per app instance**, owned by the single logged-in Mac user.
- Let the owner **create bots** from role templates or from a plain-language job description, with a name, avatar, team, and access grants.
- Organise bots into **teams**, each with exactly one **lead**; and **org-level bots** (at least the default **Chief of Staff**) that coordinate across teams.
- Provide **four kinds of conversation**: direct messages with a bot, projects, and (planned) team channels and task threads created by delegation.
- Group work into **projects**: a name, a four-part brief, a folder, a conversation of their own, and a status the app works out from the tasks.
- Let any bot with the right scope **assign work** to a team or bot below it, with the result returned into the thread the owner can read.
- Keep an **Inbox** of things that need the owner: approvals, questions from bots, finished deliverables.
- Write deliverables as **files into a Workspace folder** the owner chooses (default `~/Documents/Hearth`), one subfolder per team.
- Shape what bots produce into **reports and dashboards**: one page that opens in a browser, a PDF and a Word file, with live figures from the office's own numbers.
- **Read** the files the owner drops in: PDF, Word, Excel, PowerPoint, RTF and pictures, with text recognition done in a sandboxed service.
- Keep **memory in plain language**: a per-bot Notes list the owner can read and edit, and an org Handbook of pages shared with every bot.
- Give bots **tools** limited to: reading, writing and rendering a Markdown file to PDF and Word in granted folders, writing a deliverable, fetching web pages and searching the web, Calendar and Reminders, Contacts, running the owner's Shortcuts, and asking the owner a question. Each tool is granted per bot or per team.
- Support **recurring assignments** that run on a schedule while the app is running.
- Show **usage and cost** per bot, team and month, with budgets that pause work when exceeded.
- Support **multiple providers** through the llm library: ChatGPT plan sign-in (the only account-based sign-in; Anthropic and Google forbid subscription use from third-party apps, so those take API keys), Gemini, OpenRouter, and any OpenAI-compatible endpoint (which is how local models arrive). Model choice is presented as quality tiers, with the exact model under Advanced.
- Use the **Apple on-device model** for small private tasks where available (titles, summaries, routing hints), never as the main worker unless the owner picks it.
- Integrate with the Mac: menu bar quick-ask, global shortcut, App Intents for Shortcuts and Spotlight, Share extension, Services menu, user notifications, Quick Look of deliverables, Keychain for credentials, drag and drop of files into any conversation.
- Export and import an **org as a single file** (bots, teams, handbook, no conversations) so people can share setups.
- Be **open source**, distributed outside the App Store with notarised builds and in-app updates. (decided 2026-10-19)

#### Engineering
- Native Swift/SwiftUI app with a bundled Go core that owns providers, the agent loop, orchestration, tools and storage. (D1, decided 2026-10-19)
- Local-first: all data in SQLite and files under the user's control. No hearth server.
- Stay free, open source and locally run: OpenAI's open-source sign-in programme excludes paid or hosted apps, and that programme is what makes "Sign in with ChatGPT" possible for a non-technical audience.
- Follow the shhh conventions for Go: Makefile targets, golangci-lint, astimate ceilings, docs/ layout.

### hearth WILL NOT (in this product, not just this release)

- **Not** be built with web technologies for its UI. No Electron, Tauri, WebView-hosted React, or web-rendered chat panes.
- **Not** run on Windows, Linux, or as a web app. iOS/iPadOS is a *possible later companion*, not a port, and is not in the roadmap until the Mac app ships.
- **Not** be multi-user. No shared orgs, no accounts, no team collaboration between humans, no hearth cloud.
- **Not** host or sell model inference. No hearth API keys, no relaying through a hearth server, no "credits".
- **Not** be a coding agent. No terminal, no git, no shell tool exposed to bots. That is shhh's job; hearth may later launch shhh for a task but will never grow its own.
- **Not** be an automation platform. No node editors, no visual workflow builders, no triggers-and-actions canvas. Recurring assignments and Shortcuts cover the need; anything more is Shortcuts' job.
- **Not** expose prompt engineering. No system prompt editor on the main path, no temperature, no top-p, no sampling knobs. Advanced shows the job description as rendered and the exact model; that is all.
- **Not** let bots talk to each other without a task. No ambient chatter, no "office simulation", no bots initiating conversations with each other.
- **Not** act outside the Mac without approval by default. No standing permission to send email, post, pay, or delete is ever granted at install time.
- **Not** run unattended as a daemon in v1. Scheduled work runs while the app is open (the menu bar extra counts as open). A LaunchAgent is a possible later addition, decided separately.
- **Not** train, fine-tune or embed models, and **not** expose a vector database or retrieval settings. Memory is plain text the owner can read.
- **Not** ship a bot marketplace, store, or social features. Import/export of a single org file is the sharing mechanism.
- **Not** generate images, audio or video in v1. Voice input/output and image generation are candidates for a later release, decided separately.
- **Not** control the owner's browser or screen. No computer-use style automation in v1.
- **Not** provide a developer plugin SDK in v1. Shortcuts are the extension surface for users; MCP servers are a candidate for a later release, decided separately.

### Grey areas resolved for v1

| Question | v1 answer |
|---|---|
| Can a bot read my email? | Only through a Shortcut the owner built and granted. No direct Mail access in v1. |
| Can a bot browse a site that needs login? | No. Fetch and search only; no interactive browsing. |
| Can two teams work on one task? | Yes, through the Chief of Staff, which fans out and merges. Teams never message each other directly. |
| Can a bot create another bot? | No. Only the owner hires. |
| Can a bot change its own job description or memory notes? | Notes yes (visible, undoable); job description no. |
| Can I run everything on a local model? | Yes, via the compatible endpoint, with a visible warning on quality for lead and coordinator roles. |
| Does the app work offline? | The UI and files yes; bots need a provider, which may be local. |
