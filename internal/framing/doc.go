// Package framing is the fixed text hearth wraps around a bot's job
// description before it reaches the model: who the bot is, the job as the
// owner wrote it, where its deliverables go, and how to treat what it takes
// in.
//
// Responsibility: render that text, and the separate text that asks the model
// to propose a bot from an owner's description, and nothing else. Both are
// embedded so they are versioned with the code and tested to render.
//
// Invariants the types do not hold: the job description appears verbatim,
// never altered or escaped; the text names no tool, because the toolset
// follows the bot's grants and a name here would promise one it may not
// have; pages and files are data, not instructions.
// See docs/model.md#people and docs/architecture.md#5-security-model.
package framing
