# One employee acts: the person's checklist for the gate, the Inbox and recovery

> Snapshot from 2026-10-20: the steps a person at the screen runs to prove an employee can act, ask and recover in the real window, and the open question about the resume tail. Every result below is awaiting the person; nothing here was run with a signed-in service. Not maintained.

## 1. What is wired

The window has an Inbox row in the sidebar with the open count and an Inbox screen in the detail column. `Team > Access…` (Option-Command-A), a lock toolbar item over the conversation and the bot's context menu open the Access sheet, with a fresh form each time. Choose… in the Access sheet and in the widget path control opens the system folder panel. The Access row for deleting says files are deleted, because the core deletes outright; moving to the Trash is a later native tool.

## 2. The person's checklist

Run from a bundle built with `make app-build`, on a throwaway data folder (`open --env HEARTH_DATA_DIR=<folder>`), signed in to a service, with a bot named Dana hired and a folder granted. Each result is recorded here in a new dated note, not by editing this one.

1. **Delete with approval.** Ask Dana to write a small file and then delete it. Expect: a card in the thread asking to delete the file and the same item in the Inbox, with the sidebar row showing 1. Choose Allow once. Expect: the card shows it was allowed, the file is gone from the folder, the Inbox count falls to 0 and Dana's reply continues. Result: awaiting the person.
2. **A question back.** Ask Dana something she cannot answer without you. Expect: she asks in the thread, the same question waits in the Inbox, and her row shows she is waiting. Answer from the Inbox. Expect: the answer appears in the thread and the run continues. Result: awaiting the person.
3. **A choice by control.** Ask Dana to offer three options for a title. Expect: a control with the three options in the thread. Answer with it, then change the answer inside the grace period. Expect: the changed answer is the one Dana uses. Result: awaiting the person.
4. **Delete not granted.** Open Access (Option-Command-A), set Delete files to Not granted and Save. Ask Dana to delete a file. Expect: the tool is refused and her reply says so; the file stays. Result: awaiting the person.
5. **A killed helper.** Start a long reply, run `pkill hearthd`, watch the window. Expect: the banner that the helper restarted, and Retry on the failed reply. Choose Retry. Expect: the run continues and earlier messages are intact. Result: awaiting the person.
6. **The resumed run on ChatGPT.** After steps 1 and 2, read the reply that continued each parked run. Note whether it reads cleanly (no repeated work, no confusion about who decided) on ChatGPT. Result: awaiting the person.
7. **Choose….** In the Access sheet add a folder with Choose…; in a question that asks for a folder or file, use its Choose…. Expect: the system panel opens at the start path and the chosen path fills in. Result: awaiting the person.

## 3. Open: how a resumed run is handed its round

When a run parks for the owner and resumes, it is handed the round it parked in either as it happened (the bot's message with its calls, then a result for each) or, with the as-text option, as one note from the app. The as-text option exists for a provider that refuses a history of calls it did not make in this request. The zero value, the round as it happened, is the default today. Whether ChatGPT accepts that history and reads it cleanly is what step 6 decides; the default changes only from that result, in its own commit. Decision: open, awaiting the person.
