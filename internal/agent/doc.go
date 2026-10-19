// Package agent is where hearth runs one bot's turn: the loop, its tool
// rounds, retry on a request the provider never answered, and compaction of
// a conversation that outgrows its window.
//
// Responsibility: be the one place the core meets the loop. The loop itself
// lives in github.com/rfizzle/agentcore, shared with the other app that runs
// it; this package holds only what is hearth's own, and today that is nothing
// beyond the test that proves the module drives a tool round here with a fake
// stream function and a fake tool executor.
//
// Seams it is handed: the stream function that opens a completion over a
// message list, and the tool executor that runs one call. Both are plain
// functions so a test supplies fakes and the orchestrator supplies the
// provider and the tools.
//
// The orchestrator calls into this package to run one bot turn; it does not
// import the module's loop itself.
package agent
