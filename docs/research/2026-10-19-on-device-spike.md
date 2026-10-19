# The on-device spike: routing a request to a team, and titling, with Apple's model

> Snapshot from 2026-10-19: what was built, measured and learned using the Foundation Models framework for small private tasks. Not maintained.

## 1. Setup

- Host: macOS 27.0, Xcode 27.1, Swift 6.4. The system model reported `available` through `SystemLanguageModel.default.availability`; every Swift command ran on the Mac through the shim from the Linux VM.
- The router builds a dynamic generation schema from the team list at run time (a string that must be one of the names), because the list is not known at compile time, so a fixed `@Generable` enum cannot hold it. The model cannot answer outside the list; the answer is checked against it again anyway.
- Every entry point takes the availability as a value. Anything but `available` returns nil, and a test injects "unavailable" so the suite passes on a Mac without the model.

## 2. Routing measurement

Three sample teams (Travel, Money, Home) and ten requests, run by a small executable rather than a test: unit tests must pass with the model unavailable, and each call takes real time.

| Result | Value |
|---|---|
| Correct routes | 10 of 10 |
| First call | 1.21 s (model loads) |
| Later calls | 0.26 to 0.28 s |
| Mean over ten | 0.36 s |

The last fixture mixes two teams ("plan a weekend trip to Vermont and tell me the cost") and routed to Travel, the intended one. Ten requests is a smoke test, not an accuracy claim.

## 3. Titles and the 4,096-token session limit

The session holds 4,096 tokens, instructions and reply included, and the tokenizer is not exposed. Input is cut to its first 4,000 characters, at most about 2,000 tokens even at two characters per token, which leaves room for the rest. An 18,000-character input titled "The Quick Brown Fox" without hitting the limit. A title over six words is discarded rather than cut, since a cut title can read as nonsense; the caller falls back to the Fast tier.

## 4. Decision

D19: M3 uses the on-device router as a hint only, with the Fast tier as the fallback whenever the model is unavailable or says nothing.
