---
name: swiftui-structure
description: Use this skill for any change under App/ (the Hearth app, HearthGallery, or a package in App/Packages such as HearthKit, HearthDesign, HearthUI, HearthFeatures, HearthNative). It covers where a new view goes, the thin-client rule, package dependency direction, Observation stores and the HearthClient protocol, PreviewState and the gallery, snapshot goldens, tokens by role, AppKit escape hatches, and the accessibility checklist. Not for Go core changes (use go-structure) or proto changes.
---

# SwiftUI structure

The packages, the app target and the gallery exist, each package with one
unit and one test, and the Swift gates are real. The reasons live in
`docs/architecture.md`, section "Swift app"; this is the working form.

## The thin-client rule (reviewable)

1. The app holds no business state. Every fact about the org comes from the
   core over RPC; every change is an RPC.
2. A view model may cache, sort, filter and animate. It may not decide.
3. A rule such as "what happens when the budget is exceeded" is tested in Go.
   "The paused card shows a resume button when the task is paused" is tested in
   Swift against fixture state.
4. Only `HearthNative` does things (EventKit, Contacts, Shortcuts, Keychain,
   notifications, the on-device model). Nothing else imports it but `Hearth`.
5. If a feature seems to need logic in Swift, the contract is missing an RPC or
   an event. Change the proto, not the view.

## Packages and direction

```
App/Hearth            app target: scenes, windows, menus, helper lifecycle, DI. Nothing else.
App/HearthGallery     second app target: every component and composition in every state
App/Packages/
  HearthKit           generated client, models, HearthClient, LiveClient, PreviewClient. No SwiftUI.
  HearthDesign        tokens and primitives; imports nothing of hearth's
  HearthUI            components and compositions that know domain models
  HearthFeatures      one folder per feature: an @Observable store and its screens
  HearthNative        native adapters and the NativeToolService server
```

`Hearth` -> `HearthFeatures` -> `HearthUI` -> `HearthDesign`. `HearthKit` is
importable by everything above `HearthDesign`. A package that needs something
from a higher layer is a design error; SwiftPM refuses the import, so do not
work around it by moving code down.

## Where a new view goes

| Unit | Is | Goes in |
|---|---|---|
| Token | semantic colour, type, spacing, radius, motion, named by role, light and dark | `HearthDesign` asset catalog and `Theme` |
| Primitive | domain-free view backed by one system control or a plain shape | `HearthDesign` |
| Component | domain-aware view of one model, presentation-only state | `HearthUI` |
| Composition | two or more existing units, local state and flows, accessibility delegated to children | `HearthUI` |
| Feature | a screen or flow: a store plus the compositions it drives | `HearthFeatures` |

Reuse first: look for a unit to reuse or enhance before adding one. A primitive
names the control it wraps in its header comment and inherits that control's
keyboard and VoiceOver behaviour. Custom drawing needs a stated reason. Agree
the states and API of the shared units (tokens, primitives, `TaskCard`,
`MessageRow`, `ApprovalCard`) in conversation before building them.

## State

- One `@Observable` store per feature subscribes to its slice of the core's
  event stream, exposes view state and turns intents into RPC calls. No TCA.
- Views are functions of store state. `@State` is presentation only: disclosure
  open, hover, scroll anchor, draft text.
- Stores take a `HearthClient` in their initialiser; views get stores through
  the environment. Wiring is manual, in the app target.
- `LiveClient` speaks to the socket; `PreviewClient` serves fixtures from
  `HearthKit/Fixtures`. Previews, gallery pages, snapshots and store tests share
  those fixtures.
- Streaming deltas are applied to the store's cached list; reconnect replays
  from the last sequence number seen.

## Previews, gallery, goldens

- Every component and composition declares a `PreviewState` enum and one
  `#Preview` per state, using `PreviewClient`.
- `HearthGallery` walks every `PreviewState` in light and dark at default and
  the largest accessibility text size. A unit without a `PreviewState` fails
  the gallery build.
- Snapshot goldens live under `App/Goldens/`, one per unit, state and
  appearance. A golden changes only in a change that says why.
- Store tests assert view state against fixtures, not pixels.

## Tokens and header comments

- Colours, fonts and spacing come from tokens by role. A SwiftLint custom rule
  bans `Color(red:`, named-colour literals and `Font.system(size:` outside
  `HearthDesign`. Contrast of every pairing is checked against WCAG 2.2 AA.
- Every file starts with a header comment: purpose in a sentence or two, the
  system control it wraps (primitives), the children it composes
  (compositions), and anything non-obvious about accessibility or layout.
  Written before the body, edited with it. No status field.

## AppKit escape hatches

Allowed only in `HearthDesign` or `HearthUI`, with a header comment saying why
SwiftUI fell short: the composer (`NSTextView` for attachments, mentions,
paste) and, if needed, the transcript (`NSTableView`-backed list). Any other
`NSViewRepresentable` needs a reason in review. No `swiftui-introspect`.

## Accessibility checklist

- VoiceOver: a label, a value and the right traits on every interactive or
  informative element; decorative views hidden; a composition's order sensible.
- Full keyboard access: every action reachable and operable without a pointer,
  visible focus, no keyboard trap.
- Reduce Motion: animation has a non-motion alternative.
- Increase Contrast: nothing relies on a low-contrast tint alone.
- Dynamic Type: layout holds at the largest accessibility size; no fixed
  heights on text.
- Targets at least 24 by 24 pt.

## Before you finish

- Does any view decide something the core should? Add an RPC instead.
- Is the unit in the right package, and do imports still point down?
- Header comment written; `PreviewState` and a `#Preview` per state present.
- Tokens by role only; no raw colour, font or spacing literal.
- Any `NSViewRepresentable` is on the allowed list or justified.
- Accessibility checklist walked in the gallery, including the large text size.
- Goldens changed only where intended, with the reason in the commit.
- `make ci-swift` and `make ci`.
