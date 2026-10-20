package agent

import (
	"encoding/json"

	"github.com/rfizzle/llm"
)

// placeholder is the text the loop records for a call that waits for the
// owner. The loop's conversation ends with the turn and the sink is never
// handed it, so no model and no person reads it.
const placeholder = "waiting for the owner"

// gate asks the caller's gate whether tc waits for the owner.
func (t *turn) gate(tc llm.ToolCall) bool {
	return t.in.Gate != nil && t.in.Gate(tc.Name, json.RawMessage(tc.Arguments))
}

// park stands in for running a call that waits for the owner. The first
// such call of the turn is recorded and the loop interrupted, so the turn
// ends once the round's remaining calls are through; every waiting call is
// marked so its result reaches the sink as ErrParked rather than as the
// placeholder. The loop runs its gated calls one at a time on its own
// goroutine, the one that reads both fields after Run.
// See docs/model.md#8-approvals.
func (t *turn) park(tc llm.ToolCall) string {
	if t.gated == nil {
		t.gated = map[string]bool{}
	}
	t.gated[tc.ID] = true
	if t.parked == nil {
		t.parked = &ToolCall{ID: tc.ID, Name: tc.Name, Args: json.RawMessage(tc.Arguments)}
		t.h.Interrupt()
	}
	return placeholder
}

// onUsage tells the sink what one request used.
func (t *turn) onUsage(u *llm.Usage) {
	if u == nil {
		return
	}
	use := Usage{Prompt: u.PromptTokens, Completion: u.CompletionTokens, Cached: u.CachedTokens, Created: u.CacheCreationTokens}
	t.tell(func(k Sink) { k.Usage(t.in.Model.Provider, t.in.Model.Name, use) })
}
