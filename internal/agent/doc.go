// Package agent is where hearth runs one bot's turn: the loop, its tool
// rounds, retry on a request the provider never answered, and the caps that
// end a turn.
//
// Responsibility: be the one place the core meets the loop. The loop itself
// lives in github.com/rfizzle/agentcore, shared with the other app that runs
// it; this package builds it from the system text, the history and the
// prompt, installs hearth's executor, scrub and round cap, and reports what
// happens to a sink the orchestrator persists from. Run is the one door.
//
// Invariants the types do not hold: the wall-time cap and a cancelled
// context both stop the turn through the loop's interrupt, so the partial
// reply is kept and the conversation stays well-formed; a tool result is
// scrubbed before the sink sees it as well as before the next request. A
// call the gate picks never runs here: the first one in a round is recorded,
// the round's other calls finish, and the turn ends with ErrParked through
// the same interrupt. The sink is never handed the loop's placeholder for a
// waiting call, and the loop's own conversation is discarded with the turn,
// so nothing reads the results the interrupt makes up for it.
// See docs/model.md#4-termination-guarantees and docs/model.md#8-approvals.
//
// Seams it is handed: a stream function that opens a completion, a tool
// executor that runs one call, an approval gate, and a sink. All are plain values, so a test
// supplies a script and a fake and the orchestrator supplies the provider
// and the tools. It imports agentcore and llm and nothing of hearth's.
package agent
