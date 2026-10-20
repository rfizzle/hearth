# The one-employee slice on a Mac: the window runs, what the person still has to do

> Snapshot from 2026-10-20: what the app showed when run from its bundle on a throwaway data folder, what could not be checked without a person at the keyboard or a real sign-in, and the live checklist. Not maintained.

## 1. What was run

`make app-build` produced `dist/Hearth.app` with the helper inside. The app was opened with `open --env HEARTH_DATA_DIR=<scratch folder>` so the helper kept its database and credentials there and the real Application Support folder was never touched. The helper accepts that variable and falls back to the Hearth folder in Application Support when it is unset. Windows were captured by id with `screencapture -x -o -l`.

No sign-in was made and no provider was called. The org and the one bot were created against the helper's own contract (a second copy of the helper on the scratch folder, driven by `curl` with the Connect JSON protocol and a known token), then that copy was stopped and the app opened on the same folder. Nothing in the window could be clicked from the shell: System Events keystrokes and clicks are refused (no Accessibility), and a synthetic click landed on the terminal in front of the app, so the empty-team Hire button, the hire sheet, the sidebar row and the Settings window were not driven.

## 2. What was seen

- **No org.** The window is titled Welcome and shows the first of three steps in the 480 pt column: the flame mark, "Step 1 of 3", the heading, one sentence and a Continue button. The hello screen is gone.
- **First run found a fault.** The window said "Couldn't reach your team: org: not found". The helper answers not_found for an org that has not been named yet, and the client reported that as a failure. The client now reads it as no org; the same launch then showed Welcome.
- **Org, nobody hired.** The title is the org's name. The sidebar shows its "Your team" header alone with an icon-only add-person item beside the sidebar toggle, so the toolbar item lands over the sidebar with the primary-action placement and the fallback was not needed. The detail column says "Nobody here yet" with a prominent Hire Someone button.
- **Org, one bot, none selected.** The sidebar row shows the initial, the name and the role; the detail says "No one selected, choose someone in the sidebar".
- **One bot selected.** The title is the bot's name with its role under it, one folder item sits at the right of the toolbar, and the empty conversation says "Say hello to" the bot. This view was reached with a temporary, uncommitted line that selected the first bot, since a click was not possible.
- **Second fault found there.** The message field floated about two thirds of the way down the window, because the empty and error states of the conversation took only their natural height and the bar sat on the bottom of that smaller area. With the content told to fill the column the field sits on the bottom edge, as in the gallery.

## 3. What could not be verified here

- Return sending and Shift-Return inserting a newline in the message field (keystrokes are refused).
- Which button Return triggers in onboarding while a key field is open.
- The hire sheet, the Team menu on Command-N, the Settings window and its two panes, and the cross-fade from onboarding to the team (it needs a click through the three steps).
- Any real sign-in, any real reply, the tool rows, the file in the folder, Show in Finder, the idle reconnect and the killed helper. The relay is reset over the new client when a helper comes back, and the window shows the helper's status while there is none, but neither was watched.
- The live run's timings.

## 4. The person's live checklist

On the Mac, from `dist/Hearth.app` (or `HEARTH_HELPER=./hearthd swift run --package-path App Hearth`), with a throwaway `HEARTH_DATA_DIR` the first time so nothing real is at stake. Write what happens into a new dated note in `docs/research/`; this one is not edited.

1. Sign in: ChatGPT in the browser, or an API key. Note what the browser showed and how long it took.
2. Name the org.
3. Choose the folder (the default is `~/Documents/Hearth`; choose another for a trial).
4. Skip the search key.
5. Hire a Research Analyst from the template card.
6. Ask for a one-page brief naming one source domain.
7. Watch it stream. Note the time to the first word and to the end, and whether headings and lists read as text or show markup characters.
8. See the fetch rows in the transcript.
9. Find the file with Show Folder in Finder.
10. Paste a Brave Search key in Settings, Web Search. Ask for a brief that names no source. See search rows followed by fetch rows and a second file. Record how the two briefs compare, without and with the key.
11. Leave the app idle for three minutes, then send again. Note whether it answers and how long it took.
12. Kill the helper in the middle of a reply (`pkill hearthd`), watch the window, then send again. Note what the window said and whether the bot's folder and earlier messages were intact.

## 5. Also check while there

- Return sends and Shift-Return starts a new line in the message field; if Return behaves differently in the vertical text field, the key handling needs fixing.
- Return in onboarding with a key field open: which button it presses.
- Settings (Command-comma): both panes show the same cards as onboarding and share their state.
- Command-N opens the hire sheet only once an org exists; there is no New Window item.
- Reduce Motion: the change from onboarding to the team is a cross-fade without animation.
- Resize to the 760 by 480 minimum: the sidebar holds between 200 and 320 pt and the hire sheet scrolls.
- Known gap: a conversation opened while a run is in progress shows idle until the next run event, because the contract cannot fetch a conversation's current run.
