# The Shortcuts spike: running a user's shortcut as a tool

> Snapshot from 2026-10-19: what was built and measured when the native layer first ran a Shortcut through the `shortcuts` command line tool. Not maintained.

## 1. Setup

- Host: macOS 27.0, Xcode 27.1, Swift 6.4. `/usr/bin/shortcuts` is present. The work ran in a Linux VM; every Swift and `shortcuts` command ran on the Mac through a shim, with no screen access.
- This Mac has no shortcuts: `shortcuts list` prints nothing and exits 0. No successful run was possible, so latency of a real run, output handling of a real shortcut and the front-most-app question are all unmeasured here (section 5).

## 2. What was measured from the command line

| Command | Result |
|---|---|
| `shortcuts list` | Empty output, exit 0, about 20 ms by the shim's wall clock, 43 ms from the Swift runner. |
| `shortcuts list --show-identifiers` | Empty output, exit 0. |
| `shortcuts run <missing> -i - -o -` | Exit 1, stderr `Error: The operation couldn’t be completed. Couldn’t find shortcut`, nothing on stdout; 90 to 145 ms. The apostrophe is a typographic one, so the runner matches on `find shortcut`. |
| `shortcuts run <missing>` with no `-i` and standard input left open | Once hung until killed (about two minutes). Re-runs with standard input at `/dev/null` returned in 90 ms with the same error, and the Swift runner, which always writes its input and closes the pipe, never hung. Treat an open standard input as a hazard and always close it. |

There is no distinct exit code for "not found": it is exit 1, the same as other failures, so only the message separates them. That text is not a contract and may change with a macOS release; the failure then surfaces as a generic failure, not a wrong answer.

## 3. What the stub tests prove

The runner is tested against a stub of the process seam; no test runs the real binary.

- A run that exits 0 within the grace period returns its output, passes the input and the argument list `run <name> -i - -o -`, and reports `running` then `finished`.
- A run still going after the grace period reports `waitingForUser` and keeps going; if it is still going at the timeout it is cancelled (the stub sees the cancellation, which is what makes the real runner kill the process) and the call throws the waiting error.
- A timeout shorter than the grace period throws a plain timeout instead.
- The missing-shortcut stderr maps to a not-found error; any other non-zero exit carries its exit code and trimmed message.
- A real run of the Swift runner against the real tool (outside the tests, from a scratch program) listed nothing and reported `running` then `notFound` for a missing name, so the real process runner's pipes, input close and exit handling work.

The grace period is a heuristic: the tool gives no signal that a shortcut is blocked on a prompt, so a slow but legitimate shortcut is also reported as waiting. The timeout, not the grace period, bounds the run.

## 4. The URL scheme

Not tried. `shortcuts://run-shortcut?name=…&input=text` goes through Launch Services, so it always brings Shortcuts to the front, returns nothing to the caller without an x-callback round trip, and needs a listener to receive it. The command line tool needs none of that, so it stays the route unless the screen check below shows the tool also takes focus.

## 5. Awaiting the person at the screen

None of this can be observed from a shell. The person should create one shortcut that takes text input and returns it uppercased (Receive Text input, Change Case, Stop and Output), and one that shows an "Ask for Input" prompt, then run these in a debug build (`swift run --package-path App Hearth`, menu Debug, Shortcuts) and by hand, and write the result into a new dated note.

1. `shortcuts list` lists both names; note the time.
2. `echo hello | shortcuts run "<uppercase name>" -i - -o -` prints `HELLO`; time it with `time`. Does the Shortcuts app come to the front, show in the Dock, or stay hidden? Check with `osascript -e 'tell application "System Events" to name of first process whose frontmost is true'` just after the run.
3. The first run of a shortcut that touches Mail, Notes, Contacts or files: which TCC prompts appear, who they name (Terminal, Hearth, or Shortcuts), and whether a second run is silent.
4. `shortcuts run "<prompt name>" -i - -o -`: where does the prompt appear, does the command hang until it is answered, and does killing it (the runner sends a terminate on timeout) close the prompt?
5. In the debug window: the states shown for a quick shortcut, for the prompt shortcut (expect `waitingForUser` after 5 s and a waiting error at 30 s), and for a bogus name (expect `notFound`). `log show --last 5m --predicate 'subsystem == "com.rfizzle.hearth"'` carries the same transitions and the run time.
6. Does the app need any entitlement or Info.plist usage string to spawn the tool, once signed as the bundle (`make app-build`)?
