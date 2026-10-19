# hearth

A native Mac app where you run a small organisation of AI employees: each one has a name, a job, a team and a boss; you talk to them the way you would talk to colleagues; the work they do lands in a folder on your Mac.

Status: design complete, implementation not started. Read `docs/README.md` for the four documents that describe the product, the model, the architecture and the decisions.

Built as a SwiftUI app over a bundled Go core (`hearthd`) that uses [github.com/rfizzle/llm](https://github.com/rfizzle/llm) for providers. macOS 26 and later, Apple Silicon. MIT licence.
