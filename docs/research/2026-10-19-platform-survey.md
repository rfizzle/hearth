# 07 · macOS and Apple ecosystem integration

> Platform survey from 2026-10-19 (macOS 26, APIs, TCC, distribution). Not maintained; verify against current Apple docs before relying on a row.

Status: draft, 2026-10-19. Each item names the API, the milestone it lands in (`.plan/BACKLOG.md`), and the gotcha.

## 1. Distribution posture

Developer ID + notarisation + Sparkle, non-sandboxed, macOS 26+, Apple Silicon. This is what makes items 5, 6 and 9 below possible. A sandboxed App Store build is a later SKU that would drop them. **[D5]**

## 2. Integrations

| # | Integration | API | Milestone | Gotcha |
|---|---|---|---|---|
| 1 | Keychain for API keys and ChatGPT refresh token | Security framework, `kSecClassGenericPassword`, non-synchronisable | M0 | Never sync credentials via iCloud. The core receives them over RPC only. |
| 2 | Hearth folder in Finder | FileManager, FSEvents, security-scoped bookmarks for extra folders | M1 | If the user picks a folder in iCloud Drive, files may be evicted; use `startDownloadingUbiquitousItem` before reads. |
| 3 | Quick Look for deliverables | `QLPreviewPanel` / `.quickLookPreview` | M1 | Free for standard types; transcripts exported as `.md` or `.pdf`. |
| 4 | User notifications | `UNUserNotificationCenter`, categories with actions (Allow once / Deny / Open) | M2 | Prompt lazily at the first approval, not at launch. Group by task to avoid spam. |
| 5 | Calendar and Reminders tools | EventKit, `requestFullAccessToEvents` / `…Reminders` | M4 | TCC prompt names the bot and task in the purpose string. Write tools are "ask me" by default. |
| 6 | Contacts tool | Contacts framework | M4 | Read only. Consider the limited-access picker if macOS 26 offers it. |
| 7 | Run the user's Shortcuts as tools | `/usr/bin/shortcuts run "Name" -i in -o out` from the non-sandboxed app; `shortcuts list` to populate the allow-list | M4 | Brings Shortcuts to the front briefly; shortcuts with hidden consent prompts stall, so the tool has a timeout and the card says "Shortcuts is waiting for you". Allow-list per bot, never "all shortcuts". |
| 8 | Expose actions to Shortcuts, Siri, Spotlight | App Intents: `AskChiefOfStaff(text)`, `AssignTask(bot, brief)`, `ApprovePending`, entities for bots and teams, `AppShortcutsProvider` | M4 | Keep intents idempotent; destructive ones use `requestConfirmation`. |
| 9 | Mail | Not in v1; via a user Shortcut only | later | No public read API (MailKit is extensions only); AppleScript needs Automation consent and is slow; IMAP/Gmail API is the clean later path. |
| 10 | Share extension | `NSExtension` share target writing into an app group inbox the app drains | M4 | Lets any app "Share to Hearth" a selection, URL or file into the Chief of Staff DM. |
| 11 | Services menu | `NSServices` + `NSServicesMenuRequestor`: "Ask Hearth about selection" | M4 | Cheap; mirrors BoltAI's selection commands. |
| 12 | Menu bar extra | `MenuBarExtra` (window style) with quick-ask, Inbox count, pause-all | M4 | Focus quirks in window style; test ⌘W and Escape. Keeps schedules running when the main window is closed. |
| 13 | Global hotkey | Carbon hotkey or `NSEvent` global monitor (needs Accessibility only for the monitor; prefer Carbon `RegisterEventHotKey`) | M4 | Default ⌥Space conflicts with other apps; make it configurable, off by default. |
| 14 | Core Spotlight | `CSSearchableIndex` for conversations and deliverables; `IndexedEntity` for bots | M5 | Delete on conversation delete; index titles and summaries, not full transcripts. |
| 15 | On-device model | Foundation Models: `SystemLanguageModel.default`, `@Generable` for routing enum and titles, streaming not needed | M3 | 4,096-token session limit; availability varies; always fall back to the Fast tier. |
| 16 | Writing Tools | Free in AppKit/SwiftUI text views | M1 | Nothing to do beyond not disabling them. |
| 17 | Sparkle 2 updates | Sparkle framework, EdDSA-signed appcast on GitHub Releases | M5 | Needs the helper signed too; test a full update cycle before launch. |
| 18 | Login item (run schedules without the window) | Not in v1. `SMAppService` LaunchAgent later | later | Decided separately; changes the "runs while app is open" boundary. |
| 19 | Handoff / iOS companion / CloudKit | Not in v1 | later | Source-of-truth question (Go SQLite vs CloudKit) must be settled before any sync work. |
| 20 | Local models | Detect `localhost:11434/v1` (Ollama) and `localhost:1234/v1` (LM Studio); add as Compatible providers | M3 | Do not bundle weights or a runtime. Warn on quality for Lead and Chief of Staff roles. |

## 3. TCC prompts the user will see, in order of first use

Notifications (first approval) → Files & Folders for Documents (if the Hearth folder is there; pre-empted by letting the user pick it with an open panel) → Calendars → Reminders → Contacts → Automation for Shortcuts (when a shortcut itself targets another app, attributed to Shortcuts). Each is requested at the moment a bot first needs it, inside a task the user asked for, with a purpose string of the form "Dana (Research) needs your calendar to find free slots for *Plan offsite*".

## 4. Sandbox stance

Not sandboxed in v1. The reasons are items 7, 9 and 2 (arbitrary folder grants without bookmark ceremony). The code is written so that paths flow through one resolver and credentials through one store, so a sandboxed SKU is a configuration change plus `app-sandbox` + `inherit` entitlements on the helper, not a rewrite.
