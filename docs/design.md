# hearth · design

What hearth looks like and why. The taxonomy of units is `architecture.md` "3. Unit taxonomy"; the
flows and the vocabulary are `model.md`; the first sketch of the screens is
`research/2026-10-19-ux-sketch.md`. This file does not repeat them.

A quiet Mac app. The system draws the chrome, content is flat, and there is one accent colour: the
one the person chose in System Settings. Nothing asks to be looked at; the bots' work and the
conversation are what is on the screen. The design system is Apple's Human Interface Guidelines,
and hearth adds a name only where the system has none (D23 in `decisions.md`). Anything a person
could mistake for a different app is a bug. No custom fonts, no coloured chrome (it hides the
user's accent), no glass on content, no bubbles for bots, who are colleagues speaking in a thread,
and no snapshot goldens before the owner approves the look (D26 in `decisions.md`).

## Materials

Liquid Glass is the system's material for the layer that floats above content: the sidebar, the
toolbar, sheets and menus. Hearth gets it by using the system's windows, toolbars and sheets and
drawing nothing behind them. Custom glass appears in one place, the composer bar floating over the
transcript, which applies it itself. Content (messages, cards, rows) is flat.

## Type

System text styles by role, scaled by Dynamic Type; no point sizes outside the design package.

| Role | Style | For |
|---|---|---|
| Large title | large title | The welcome mark, once |
| Title | title 2 | A screen's or sheet's heading |
| Headline | headline | A name, a card title |
| Body | body | The transcript, fields, rows |
| Callout | callout | A row's second line, a tool summary |
| Caption | caption | A time, a step counter, a pill |
| Monospaced | body, monospaced | Tool arguments and results |

## Colour

Most colour is the system's (the user's accent, the separator, the window background). Hearth
names a colour only for what it draws itself.

- **Neutral surfaces and text.** Primary and secondary text, and the primary, secondary and card
  surfaces: fixed light and dark pairs rather than the system's label colours, because the build
  proves the contrast of a pair and cannot prove it for a colour that carries transparency.
- **Three tones.** Positive (ready, done), attention (waiting on the person) and negative (stopped,
  failed), each a text colour with a quiet fill behind it. A neutral tone is secondary text on the
  secondary surface. A view picks a tone, never a colour.
- **Never colour alone.** A tone always travels with a word or a label that says the same thing, so
  Increase Contrast, colour blindness and VoiceOver lose nothing.
- **Checked.** Every foreground and background used together is listed in the token file, and the
  build fails when one falls under 4.5 to 1 (WCAG 2.2 AA) in light or dark. The owner's message
  uses the secondary surface, not the accent, so the person's accent never decides readability.
- **Six chart hues.** A shaped deliverable is cards in a grid on the content layer; its charts use
  six fixed hues, each checked at 3 to 1 against the card in light and dark and named in a legend.
  A dashboard prints landscape and a report portrait.

## Spacing and shape

Six spacing steps (4, 8, 12, 16, 24, 32 pt) and three corner radii (6, 10 and 16 pt), continuous,
and capsules for pills. Every padding, gap and radius outside the design package comes from these;
a number that fits none is a reason to change the scale, not to skip it.

Shared measures: the avatar (28 pt, 44 pt large), the status dot, a project's progress pie (16 pt),
the file icon, a 24 pt minimum target, a readable column of 640 pt, 480 pt for a bubble and a form, 560 for a sheet.

## Window anatomy

One main window with two columns: the team in the sidebar, the conversation beside it. The window
is titled by what is selected, never by the app's name. Every toolbar button is also a menu
command, so the keyboard reaches all of it. A flame in the menu bar (filled while anyone works,
dotted while anything waits, off in Settings) opens a plain one-column panel, a quick ask over who
is working and what needs you, and ⌃⌥Space opens the quick ask alone from any app.

- **Sidebar.** "Your team": one row per bot with its mark, name and role. Then "Projects": the
  active ones, then the done ones, each with a small pie of its tasks done as the mark, its name,
  and its light with the light's word. The system gives it its glass and vibrancy.
- **Hire and New Project.** Toolbar buttons over the sidebar, each with a menu command of the same
  name (Team, File). Not at the bottom of the sidebar: critical actions do not live there.
- **Inspector.** Opens at the trailing edge (⌥⌘I) with collapsible sections about the selected
  bot: Notes first (a note is pinned or edited in place), then the Handbook and Usage. Usage
  headlines the month's cost in dollars, with tokens under a disclosure, a quiet budget bar, and
  Paused with Resume.
- **Conversation.** The transcript in a readable column; its one toolbar button shows the bot's
  folder in Finder. The composer is a bar of glass floating over the transcript's lower edge, one
  signal of work in progress above it, and Stop in the bar rather than the toolbar.

## The transcript

A bot's message is flat on the content layer with its mark, name and time, like a thread in a mail
client. The owner's message is a quiet grey bubble on the right, as in Messages (D25 in
`decisions.md`); the asymmetry makes who is speaking clear without reading a name.

Tool calls are collapsed rows indented under the bot's turn, one line saying what was done;
opening one shows its arguments and result. Files a bot made are chips with the Finder's own icon
for their type. There is one signal that the bot is working, a status line with a pulsing dot
(steady under Reduce Motion), and no streaming cursor. A run that failed or was stopped puts a
plain Retry beside its line. When hearth's helper stops and starts again, a bar on the attention
fill sits over the top of the conversation saying what stopped, with OK; it stays until dismissed
and does not apologise.

Files go both ways. A file dropped on the composer waits there as a chip with a small remove
button; a folder is declined in a line under the chips. Any file chip opens in Quick Look on Space
and from its context menu. A summary of earlier turns is a quiet row above the oldest message,
"Earlier in this conversation", that opens to show it.

A finished run folds into one quiet line, "Worked for 4m 12s", closed. Opened, it shows what the
bot said as plain text and each unbroken burst of tool calls as one line with one icon, counted by
rule ("Read 3 files, searched the web, wrote 1 file"); the files a burst made stay under its line.
Opening moves with the layout move, so the answer below slides rather than jumps. A run with no
known times reads "Worked on this".

The transcript follows new messages while the bottom is in view and stops once the person scrolls
up. Then a round glass button floats above the composer to jump to the latest message. While the
bot is still writing it widens into a capsule that says "Still writing" beside the arrow, so work
is not missed and the person is not pulled down to it.

A chart is a card with hover callouts. Its menu has Expand, Copy Data (tab-separated text that
Numbers and Excel paste as a table) and Save Image (a PNG at twice the size). Expanded, it grows
out of its card over a dimmed backdrop, up to 1000 points wide, and closes on a button, Escape or a
click outside; under Reduce Motion it fades in instead.

A conversation that produced files gets a small glass card at the top edge of its transcript: the
deliverables as chips, then other files as rows, five at most with "Show all". The header folds it
into one pill, "Produced 4 items", and back; the fold moves with the quick token.

A bot's question is a card with the question, the control it asked for (a choice, a date, a
folder) and one prominent button to answer. An answered card goes quiet: no stroke, no buttons,
the choice in words where the control was.

## Forms, sheets and settings

Forms are the system's grouped form. A sheet has a heading, a lead sentence, the form, and a row
of buttons: Cancel on the left bound to Escape, and the one prominent button bound to Return, never
two prominent buttons. Hiring and a new project open instead as one question from the Chief of
Staff with a text box; a draft fills a card the person edits, with at most three concerns beside
its fields, and "Just show me the form" is remembered per sheet. Settings is the standard settings
window (Command-comma) with a tab per pane: Accounts, Models, Web Search, Usage. Models is a row
per tier with a menu of the models it may use; a bot's own model is pinned under Advanced in its Access sheet.

## Empty states and first run

An empty screen says what is missing and offers the one thing to do about it, in plain words
("Nobody here yet. Hire someone to get started."), with the system's empty-state layout. Nothing
apologises and nothing lists features. The one exception is an empty team or project list, which
offers a few example cards: a symbol, a title and one line, such as "Research a market". Pressing
one opens the usual setup with the example filled in and creates nothing.

A conversation with a bot opens with the bot's introduction on a card, not in the flat transcript:
who it is, what it is here for, three lines on how to work with it and three things to try, the
last as grey chips that fill the message field and never send. Once the conversation has messages
the card collapses to an "About Dana" row pinned under the title, which opens into the same card.

First run happens inside the main window: three steps in a centred column, with no sidebar and no
toolbar, and a cross-fade to the split view once the organisation exists (D24 in `decisions.md`).
Optional setup, such as web search, waits, collapsed, until after the first conversation.

## Motion

Two durations, named by what they are for: quick (0.2 s, ease-out) for feedback that follows a
click, a swap or a fade; move (0.35 s, a spring with no bounce) for a layout change, a height
growing or a row sliding in. Nothing else is written inline.

What animates is a change the person asked for. Content that arrives on its own (streamed text,
events from the core, a status flipping) appears, so the transcript never jitters.

Reduce Motion: every animation passes through one helper that returns nothing under the setting. A
change still happens, at once, or as a cross-fade where a jump would lose the person's place.
Repeating effects, such as the working dot, stop; their labels stay.

The transitions in use, one each: a disclosure grows with its body fading in; a revealed field
slides down; a pill or swapped button blur-replaces in place; a step change cross-fades while the
column's height moves; the owner's sent message rises in; a new team member slides into the
sidebar; an empty state fades out; a symbol that changes meaning replaces itself. Nothing travels.
