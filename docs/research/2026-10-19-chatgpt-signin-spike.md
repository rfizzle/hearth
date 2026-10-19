# The ChatGPT sign-in spike: the helper signs in with a plan, hermetically proven, live half still the person's

> Snapshot from 2026-10-19: what the helper can do about signing in with a ChatGPT plan, what the tests prove, and what only a person at the screen can finish. Not maintained.

## 1. What was built

`hearthd` has three commands over `github.com/rfizzle/llm` v0.7.0:

- `hearthd login chatgpt` runs the browser sign-in: it opens the authorize page in the default browser (`open`), listens on `127.0.0.1` for the callback and saves the sign-in.
- `hearthd logout chatgpt` revokes the refresh token at the server and clears the tokens.
- `hearthd say <message>` sends one message through the stored sign-in on the plan route (Responses API with `store: false` and `stream: true`, which the route sets itself) and prints the streamed reply.

The sign-in is kept in `~/Library/Application Support/Hearth/chatgpt/`, a 0700 directory of 0600 files, by llm's file store (since v0.7.0 in its own `chatgpt/credstore` package). That is deliberate for the spike: the tokens sit on disk until a Keychain-backed store replaces it. hearth names itself to llm as `hearth`, prefix `HEARTH`, with the hint "Settings → Accounts → Sign in with ChatGPT" for the app's UI to show; the helper's own signed-out message is one line that names `hearthd login chatgpt`.

## 2. What the hermetic test proves

Against llm's fake authorization server pattern, with the loopback listener held in memory (no socket, no network, no window):

- Before any sign-in the plan provider cannot be built, and the failure matches both llm's signed-out error and the chatgpt package's alias of it.
- A sign-in through the fake browser completes the callback, and the account is reported.
- The directory is 0700 and every file in it owner-only.
- A second account opened on the same directory (the relaunch) returns the stored access token with no browser and builds the provider.
- Sign-out reaches the revocation endpoint once, and afterwards the provider cannot be built; a second sign-out reports signed out.

It does not prove anything about the real service: the discovery document, the dynamic client registration, the consent page, the real token lifetimes and the plan's model catalog are all faked or absent.

## 3. What llm's documentation says the app name needs

No manual registration. The first sign-in on a host registers the app dynamically: the authorize request carries an `agent_name_hint` (the app name, `hearth`) and a per-host id, and the server hands back a client id that the helper keeps and reuses. Nothing is to be registered with OpenAI ahead of time under the hearth name. What the person should check is that the consent page names the app `hearth`, and that the second sign-in on the same host does not ask to register again.

## 4. The live run, for the person

On the Mac, in a checkout, with a ChatGPT plan that allows Codex-style use:

```
make build
./hearthd login chatgpt        # a browser tab opens; approve it
ls -ld ~/Library/Application\ Support/Hearth/chatgpt
ls -l  ~/Library/Application\ Support/Hearth/chatgpt
# quit the terminal and open a new one, so nothing is carried over
./hearthd say "hello"          # streams a reply with no browser
./hearthd logout chatgpt
./hearthd say "hello"          # one line naming `hearthd login chatgpt`
```

## 5. What to observe, and write into a new dated note

Write a new file in `docs/research/` with the day's date (this one is not edited), recording:

- What the browser tab showed: the name the consent page gave the app, the wording of the permissions, whether the plan was named, and what the loopback page said after approval.
- Timing: seconds from `login` to "Signed in", and from `say` to the first word and to the last, cold and then warm.
- Whether the second `login` after a `logout` asked to register again or reused the client id.
- The modes of the directory and its files as listed above (expected 0700 and 0600).
- Whether `say` after a relaunch needed any sign-in, and whether a refresh happened (the access token lasts an hour, so wait one to see it).
- The exact text of any refusal: a plan without access, a revoked sign-in, the consent page declining the name.
- Whether `open` was enough as the browser hook or the default browser was not the one wanted.
