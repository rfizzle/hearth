package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	coreagent "github.com/rfizzle/agentcore/agent"
	"github.com/rfizzle/llm"
)

// replay streams one scripted reply per request, in order, and records the
// messages each request carried.
type replay struct {
	replies [][]llm.StreamEvent
	seen    [][]llm.Message
}

func (r *replay) stream(msgs []llm.Message, _ string) (<-chan llm.StreamEvent, context.CancelFunc, error) {
	r.seen = append(r.seen, append([]llm.Message(nil), msgs...))
	events := r.replies[len(r.seen)-1]
	ch := make(chan llm.StreamEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch, func() {}, nil
}

func TestTurnRunsOneToolRound(t *testing.T) {
	call := llm.ToolCall{ID: "c1", Name: "lookup", Arguments: `{"q":"hearth"}`}
	fake := &replay{replies: [][]llm.StreamEvent{
		{{Token: "looking", ToolCalls: []llm.ToolCall{call}}, {Done: true, ToolCalls: []llm.ToolCall{call}}},
		{{Token: "found it"}, {Done: true}},
	}}

	a := coreagent.New([]llm.Message{{Role: llm.RoleSystem, Content: "sys"}}, fake.stream)
	var ran []string
	a.SetExecutor(func(_ context.Context, name string, args json.RawMessage) (string, error) {
		ran = append(ran, name+" "+string(args))
		return "result-text", nil
	})

	h := &coreagent.Headless{Agent: a}
	final, err := h.Run("find it")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if final != "found it" {
		t.Errorf("final = %q, want %q", final, "found it")
	}
	if len(ran) != 1 || ran[0] != `lookup {"q":"hearth"}` {
		t.Errorf("executor calls = %q, want one lookup", ran)
	}
	if len(fake.seen) != 2 {
		t.Fatalf("stream opened %d times, want 2 (the call, then the answer)", len(fake.seen))
	}

	var sawResult bool
	for _, m := range fake.seen[1] {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "result-text") {
			sawResult = true
		}
	}
	if !sawResult {
		t.Errorf("second request did not carry the tool result: %+v", fake.seen[1])
	}
}
