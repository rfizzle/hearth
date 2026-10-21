# Codex in the ChatGPT app

> Snapshot from 2026-10-21: a hands-on walkthrough of the Codex mode in ChatGPT for macOS (26.1013), made when the owner asked what feels good in it and what hearth could take. Not maintained. The app is a Chromium shell, driven here through the accessibility tree and synthetic pointer events, with stills and screen recordings. Two harmless prompts were sent from a chat with no project; nothing was accepted, connected or changed. Durations are judged from frames, not measured: the recorder writes a frame only when the screen changes, and its clock was not reliable enough to quote milliseconds.

## Layout

- A slim icon rail (Home, Space, Scheduled, Plugins, Explore, Code Review, Sites, profile at the foot) beside a sidebar. The sidebar groups threads under pinned project folders, five rows each with "Show more", then a flat Recents list. Hovering a recent row shows Pin and Archive buttons in the row.
- The empty state is one icon and one line, "What should we build?", which names the project once one is chosen ("What should we build in vault?").
- The composer sits low and wide. Above it, a tray of chips: project, where the work runs (This computer, Cloud, Remote), environment. Inside it: attach, the approval mode, the model and effort, voice, and a round send button that becomes a stop square while a reply is written.
- A small floating card at the top right of a thread summarises what the thread produced: "Changes +1,130 −4" and a link to the merge request on a code thread; "Outputs" and "Sources", each with a plus, on a chat.

## The activity trace

A finished reply leads with a quiet "Worked for 11m 50s ›" row. Expanded, it is the agent's own narration in plain prose ("I'll inspect register.js and the chart data it defines…"), with each burst of tool use folded into one grey line with an icon: "Read files, ran a command", "Edited files, read files, ran commands". The answer follows the trace, then a file-change card ("Edited register.js +135 −0" with Undo and View changes), then a row of copy, read aloud, feedback and more, with the time on hover.

Collapsing fades the trace out while the answer slides up into its place; expanding reverses it. Both are short, a handful of frames. The bottom of a long thread fades under the composer, and a round down-arrow button jumps to the latest message. While a reply is being written and the reader has scrolled away, that button shows a three-dot pulse instead of the arrow, so the reader knows work continues without being pulled down to it.

## Charts and rich output

Asked for a bar, a line and a pie chart and a table, Codex answered in about a minute with Mermaid diagrams drawn inline:

- **Kinds**: bar and line (one or more series), pie with a legend, flowchart. A table renders as plain markdown with hairline rows.
- **Hover**: none. No tooltip, no highlight, no value on any mark, and no pointer change.
- **Per chart**: on hover, a more button (Copy as code, Download image) and an expand button.
- **Expand**: a light box over a dimmed window, with round download and close buttons and a zoom pill (− 100% +). It cuts in with no transition: the dim and an empty sheet appear first and the chart a frame later.
- **Legibility**: the default palette drew a one-series line chart in a pale cream line that is barely visible on white; the bars are the same cream. A two-series line chart (planned and actual) had no legend, so the series cannot be told apart. The flowchart laid a plan-do-check-adjust cycle out right to left from "Check", with a loop line over the top.

## Other surfaces

- **Space**: a template gallery ("Create together in Space") of page thumbnails: project tracker, to-do list, feedback tracker, weekly update; tabs for suggested, favourites, yours and shared; a list of items below.
- **Scheduled**: an empty state with a clock, one line ("ChatGPT can take care of ongoing tasks so you don't have to"), and six example task cards with an emoji, a title and one line each (email monitor, track industry news, weekly finances update).
- **Plugins**: connectors in a two-column list with a coloured logo, a name, one line, and a plus; installed ones listed in the sidebar.
- **Code Review**: before GitHub is connected, a centred call to action over a slow ambient animation of falling characters.
- **Sites**: skeleton rows while loading, then a terms dialog before first use.

## Against hearth

| Area | Codex | hearth today |
|---|---|---|
| Chart hover | none | callout with every series' value, pie slice and percentage (S-0118) |
| Chart legend and contrast | missing legend on two series; pale default line | Swift Charts with the design palette and legends |
| Chart expand | light box with zoom and download, no transition | none; the deliverable window shows the whole document |
| Chart menu | copy as code, download image | none per chart; the deliverable exports PDF and DOCX |
| Activity | one "Worked for" disclosure; narration plus folded step lines | one disclosure row per tool call |
| Following a reply | stays put when scrolled up; jump button shows a pulse while writing | always anchored to the bottom |
| Thread summary | floating card of changes or outputs | files and deliverables appear as chips in the messages |
| Empty states | one line plus example cards to start from | plain empty states |
| Templates | gallery with thumbnails | templates listed in Settings |
| Motion | short fades and height changes; none on the light box | `quick` 0.2 s ease-out and `move` 0.35 s smooth, every animation through one gate that honours Reduce Motion |

## What feels good, ranked for hearth

1. **One quiet disclosure per run.** "Worked for 4m 12s" collapsed once the run ends, with the bot's narration in prose and each burst of tool calls folded into one line named by kind ("Read 3 files, searched the web"). The step line can be built from tool kinds by rule, not by the model. It reads like a colleague's note, where a row per call reads like a log.
2. **A jump-to-latest button that doubles as the working signal.** Stop forcing the view to the bottom when the reader has scrolled up; show the button, with a pulse while the bot is still writing.
3. **Expand a chart.** A larger view of one chart with the same hover callouts, opened with a zoom transition from the card (the motion Codex lacks), and a small menu: copy the data as a table, save the image.
4. **A thread summary card.** In a task thread, a small card that lists what the work produced so far: deliverables, files changed, spend. It answers "what did I get" without scrolling.
5. **Example cards in empty states.** For schedules, teams and projects, a short line and four to six example cards that start the guided setup with the example filled in. This fits the guided setup in E-008.
6. **Template thumbnails.** Show a small rendering of each deliverable template where a template is chosen, not only its name.

Not worth taking: Mermaid-drawn charts (generic look, no interaction, legibility depends on the theme), the light box's hard cut, and an ambient animation behind a call to action, which competes with the content and needs a Reduce Motion path anyway.

## Not covered

Dark mode (switching it would change a system setting), the diff viewer behind "View changes", Code Review and Sites past their first screen, the voice composer, and exact animation timing.
