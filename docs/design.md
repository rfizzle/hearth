# hearth · design

What hearth looks like and why. The taxonomy of units is `architecture.md`
"3. Unit taxonomy"; the flows and the vocabulary are `model.md`; the first
sketch of the screens is `research/2026-10-19-ux-sketch.md`. This file does not
repeat them.

## The look in one paragraph

A quiet Mac app. The system draws the chrome, content is flat, and there is one
accent colour: the one the person chose in System Settings. Nothing in hearth
asks to be looked at; the bots' work and the conversation are what is on the
screen. The design system is Apple's Human Interface Guidelines, and hearth adds
a name only where the system has none (see D23 in `decisions.md`). Anything a
person could mistake for a different app is a bug.

## Materials

Liquid Glass is the system's material for the layer that floats above content:
the sidebar, the toolbar, sheets and menus. Hearth gets it by using the system's
windows, split views, toolbars and sheets and by drawing nothing behind them.
Custom glass appears in exactly one place hearth owns, the composer bar floating
over the transcript, because that is the one control that sits over content and
should let it show through. Content itself (messages, cards, rows) is flat. Glass
on content competes with the text it sits on and, used everywhere, stops meaning
"this floats". There is no custom glass primitive: the composer applies it
itself, so glass has exactly one owner.

## Type

System text styles by role, scaled by Dynamic Type. No custom fonts and no point
sizes anywhere outside the design package.

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

Most colour is the system's: the accent is the user's accent, hairlines are the
system separator, the window background is the system's. Hearth names a colour
only for what it draws itself.

- **Neutral surfaces and text.** Primary and secondary text, and the primary,
  secondary and card surfaces. These are fixed light and dark pairs rather than
  the system's label colours, because the build proves the contrast of a pair and
  cannot prove it for a system colour that carries transparency.
- **Three tones.** Positive (ready, done), attention (waiting on the person) and
  negative (stopped, failed), each a text colour with a quiet fill that sits
  behind it. A neutral tone is secondary text on the secondary surface. A view
  picks a tone, never a colour.
- **Never colour alone.** A tone always travels with a word or a label that says
  the same thing, so Increase Contrast, colour blindness and VoiceOver lose
  nothing.
- **Checked.** Every foreground and background that is used together is listed in
  the token file, and the build fails when one falls under 4.5 to 1 (WCAG 2.2
  AA) in light or dark.

The owner's message uses the secondary surface, not the accent, so what the
person chose in System Settings never changes whether a message is readable.

## Spacing and shape

Six spacing steps (4, 8, 12, 16, 24, 32 pt) and three corner radii (6, 10 and
16 pt), and capsules where a shape is a pill. Corners are continuous. Every
padding, gap and radius outside the design package comes from these; a number
that fits none of them is a reason to change the scale, not to skip it.

A few measures are shared: the avatar (28 pt, 44 pt large), the status dot, the
file icon, a 24 pt minimum target, a readable column of 640 pt for the
transcript, 480 pt for a message bubble and a form, 560 pt for a sheet, and a
sidebar of 200 to 320 pt. Long text never runs the width of a wide window.

## Window anatomy

One main window with two columns: the team in the sidebar, the conversation
beside it. The window is titled by what is selected, never by the app's name.
An inspector opens at the trailing edge (260 to 380 pt, ⌥⌘I): collapsible
sections about the selected bot, Notes first, then the Handbook and Usage; a note is
pinned or edited in place. Usage headlines the month's cost in dollars, with
tokens under a disclosure, a quiet budget bar, and Paused with Resume.

- **Sidebar.** "Your team": one row per bot with its mark, name and role. The
  system gives it its glass and vibrancy.
- **Hire.** A toolbar button over the sidebar and a Team menu command with the
  same name. Not at the bottom of the sidebar: critical actions do not live
  there.
- **Conversation.** The transcript in a readable column, the composer bar below
  it. Its one toolbar button shows the bot's folder in Finder.
- **Composer.** A bar of glass floating over the transcript's lower edge, one
  signal of work in progress above it, and Stop in the bar rather than the
  toolbar.

Every toolbar button is also a menu command, so the keyboard reaches all of it.

## The transcript

A bot's message is flat on the content layer with its mark, name and time, the
way a thread reads in a mail client: the text is the content and needs no
container. The owner's message is a quiet grey bubble on the right, the
convention Messages taught everyone (D25 in `decisions.md`). The two are
asymmetric on purpose, so who is speaking is clear without reading a name.

Tool calls are collapsed rows indented under the bot's turn, one line saying what
was done; opening one shows its arguments and result. Files a bot made are chips
with the Finder's own icon for their type. There is one signal that the bot is
working, a status line with a pulsing dot (steady under Reduce Motion), and no
streaming cursor. A run that failed or was stopped puts a plain Retry beside
its line; one click runs the last request again. When hearth's helper stops and
starts again, a bar on the attention fill sits over the top of the conversation
saying what stopped, with OK. It is not a modal, it stays until dismissed, and
it does not apologise.

A bot's question is a card with the question, the control it asked for (a
choice, a date, a folder) and one prominent button to answer, so choosing is
a click rather than a sentence. An answered card goes quiet: no stroke, no
buttons, the choice in words where the control was.

## Forms, sheets and settings

Forms are the system's grouped form. A sheet has a heading, a lead sentence, the
form, and a row of buttons: Cancel on the left bound to Escape, and the one
prominent button bound to Return, never two prominent buttons. Hiring is a sheet
at a fixed width and no fixed height. Settings is the standard settings window
opened with Command-comma, with a tab per pane (Accounts, Web Search, Usage). Nothing in it is specific to
first run.

## Empty states and first run

An empty screen says what is missing and offers the one thing to do about it, in
plain words ("Nobody here yet. Hire someone to get started."), with the system's
empty-state layout. Nothing apologises and nothing lists features.

First run happens inside the main window: three steps in a centred column, with
no sidebar and no toolbar, and a cross-fade to the split view once the
organisation exists (D24 in `decisions.md`). Optional setup, such as web search,
is collapsed and waits until after the first conversation.

## Motion

Motion follows Apple's Human Interface Guidelines page on motion: purposeful,
brief, and never in the way. Two durations, named by what they are for: quick
(0.2 s, ease-out) for feedback that follows a click, a swap or a fade; move
(0.35 s, a spring with no bounce) for a layout change, a height growing or a row
sliding in. Nothing else is written inline.

What animates is a change the person asked for. What does not is content that
arrives on its own (streamed text, events from the core, a status flipping); it
appears, so the transcript never jitters while a reply streams.

Reduce Motion: every animation passes through one helper that returns nothing
under the setting, so nothing animates by accident. A change still happens, at
once, or as a cross-fade where a jump would lose the person's place. Repeating
effects, such as the working dot, stop; their labels stay. The system's motion
stays the system's: glass, sheets, scroll edges, selection and hover.

The transitions in use, one each: a disclosure grows with its body fading in; a
revealed field slides down from the top; a state pill or a swapped button
blur-replaces in place; a step change cross-fades while the column's height
moves; the owner's sent message rises in and the newest line stays in view; a
new team member slides into the sidebar; an empty state fades out and the
content is simply there; a symbol that changes meaning (send, stop) replaces
itself. Nothing travels between two places. No motion library is needed.

## What stays out

- **Custom fonts.** The system's type scales, localises and is what people expect.
- **Coloured chrome.** A tinted toolbar or sidebar hides the user's accent and the
  system's materials.
- **Glass on content.** See Materials.
- **Bubbles for bots.** Bots are colleagues speaking in a thread, not a chat
  partner on the other side of a screen.
- **Snapshot goldens before approval.** A golden freezes a look, so they follow
  the owner's approval of it (D26 in `decisions.md`).
