# hearth

A native Mac app where you run a small team of AI employees for your work. Each one has a name, a job, a team and a boss. You talk to them the way you would talk to colleagues, they ask before they do anything that matters, and what they make lands as files in a folder on your Mac.

hearth is for one person at work at a small company: a founder, or anyone who wants a research analyst, a writer, a marketing lead and a finance analyst without hiring them. It asks for no prompt writing and no config files.

## What works today

- **A Chief of Staff from the first minute.** Sign in, name your org, choose a Hearth folder, and the Chief of Staff answers in a direct message: it handles small things itself and suggests who to hire for the rest.
- **Hiring by conversation.** Describe the job in a paragraph, or start from a role template (Research Analyst, Writer, Planner, Marketing Lead, Finance Analyst). The Chief of Staff drafts the role, points out what is vague, and you edit a card before anyone is hired.
- **Projects.** A project has a goal, a "done when", a target date, a folder and its own conversation. Its status comes from rules, not from a model's guess.
- **Access you can read.** Each employee has a checklist of what it may do: allowed, ask me first, or not granted. Anything that writes, deletes or reaches outside asks you in the Inbox, in plain English.
- **Files in, files out.** Employees read PDF, Word, Excel, PowerPoint, RTF, CSV and images (text recognition runs in a separate sandboxed service). They write Markdown, and turn it into a PDF or a Word file.
- **Reports and dashboards.** An employee can produce a shaped deliverable: cards, charts, tables and progress in a grid, opened in its own window, saved as a web page, a PDF and a Word file. Dashboards read live figures from the app, such as spend and project progress. Charts show values on hover, expand, and copy their data.
- **Memory you control.** Each employee keeps notes you can read and edit, and an org Handbook holds pages every employee reads.
- **Spend you can see.** Usage and cost per employee and per month, with budgets that pause work before they are passed.
- **Part of the Mac.** A menu bar extra for a quick question, an optional global hotkey, Quick Look, Finder and drag and drop.

**Providers:** sign in with a ChatGPT plan, or add API keys for Anthropic, Gemini, OpenRouter or any OpenAI-compatible endpoint, which is how local models arrive. Models are chosen as quality tiers, with the exact model under Advanced.

## What is coming

- **Teams and delegation:** team channels, a lead per team, and tasks the Chief of Staff hands down and tracks.
- **Schedules:** recurring work such as a Monday status report.
- **More of the Mac:** Calendar, Reminders, Contacts, Shortcuts, App Intents and a Share extension.
- **Releases:** signed, notarised builds that update themselves. Until then, build from source.

## Build from source

You need macOS 26 or later on Apple Silicon, Xcode 26 or later, Go (the version in go.mod), and SwiftLint (`brew install swiftlint`).

```
make app-build      # builds dist/Hearth.app with the core and the reading service inside
make app-sign       # ad-hoc signs it when no signing identity is set
open dist/Hearth.app
```

For development, `make help` lists every target. The checks CI runs are:

```
make ci             # Go: format, vet, lint, tests, docs, contract, tokens, astimate
make ci-swift       # Swift: format, lint, tests, and the gallery build (macOS only)
```

`make gallery-open` opens the gallery, which shows every component in every state in light and dark.

## How it is built

A SwiftUI app over a Go core, `hearthd`, which the app starts as a helper and talks to over Connect-RPC on loopback. The core owns the providers (through [github.com/rfizzle/llm](https://github.com/rfizzle/llm)), the agent loop, the tools, the org and its storage in SQLite. The app owns the windows, the Keychain and everything that needs macOS.

## Documents

- `docs/README.md` explains the documents: the product, the model, the architecture, the design and the decisions.
- `AGENTS.md` is the map of the code, the commands and the traps, for people and coding agents.

## Licence

MIT. See `LICENSE`.
