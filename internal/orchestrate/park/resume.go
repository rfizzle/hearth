package park

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
)

// Note is the prompt of a run that continues a parked one. It is the app's,
// not the owner's, and the framing tells the bot so.
const Note = "(hearth) The owner has decided; the result is above. Carry on with what you were doing."

// NotRun is the result a resumed run reads for a call of the parked round
// that never ran: one parked behind the call the owner decided on.
const NotRun = "not run: the conversation moved on"

// Resume is how a resumed run is handed the round it parked in. The zero
// value hands it the round as it happened: the bot's message with its calls,
// then a result for each. AsText hands it the same round written out in one
// note from the app instead, for a provider that will not take a history of
// calls it did not make in this request.
type Resume struct {
	AsText bool
}

// Round is the tool round a run parked in, as the thread holds it: the seq
// it starts at, what the bot wrote before its calls, the calls in the order
// they were stored, and the result stored for each, by call id.
type Round struct {
	Start   int64
	Text    string
	Calls   []org.ToolCall
	Results map[string]string
}

// Find is the round the run runID parked in, from msgs, oldest first. The
// round ends at the request the run parked on and runs back over the calls
// that waited, the results of the calls that ran, those calls, and the text
// before them, stopping at anything else: the result of an earlier round's
// call is where that round ends. Results are read from the whole run, since
// the decided call's result is stored after the request.
func Find(msgs []org.Message, runID string) (Round, bool) {
	end := -1
	for i, m := range msgs {
		if m.RunID == runID && (m.Kind == org.MessageApprovalRequest || m.Kind == org.MessageQuestion) {
			end = i
		}
	}
	if end < 0 {
		return Round{}, false
	}
	first := roundStart(msgs, end, runID)
	r := Round{Start: msgs[first].Seq, Results: map[string]string{}}
	for _, m := range msgs[first:end] {
		switch m.Kind {
		case org.MessageText:
			r.Text = m.Content
		case org.MessageToolCall:
			r.Calls = append(r.Calls, m.ToolCalls...)
		}
	}
	for _, m := range msgs {
		if m.RunID == runID && m.Kind == org.MessageToolResult {
			r.Results[m.ToolCallID] = m.Content
		}
	}
	return r, len(r.Calls) > 0
}

// roundStart is the index of the first message of the round that ends at
// the request msgs[end]. Back from the request come the calls that waited,
// the results of the calls that ran, those calls, and the text before them.
func roundStart(msgs []org.Message, end int, runID string) int {
	first, sawResult, ranCalls := end, false, false
	for i := end - 1; i >= 0; i-- {
		m := msgs[i]
		switch {
		case m.RunID != runID:
			return first
		case m.Kind == org.MessageText:
			return i
		case m.Kind == org.MessageToolCall:
			ranCalls = sawResult
		case m.Kind == org.MessageToolResult && !ranCalls:
			sawResult = true
		default:
			return first
		}
		first = i
	}
	return first
}

// Assemble is the history and the prompt of a resumed run: the text before
// the round, then the round, then note. The round is the bot's message with
// its calls and one tool message per call, or with AsText one note from the
// app that the prompt follows. A bot message that ends the text history is
// joined to the round's, since some providers refuse two turns in a row
// from one side.
// See docs/model.md#2-a-run.
func (rs Resume) Assemble(history []agent.Message, r Round, note string) ([]agent.Message, string) {
	if rs.AsText {
		return history, asText(r) + "\n\n" + note
	}
	bot := agent.Message{Role: agent.RoleBot, Text: r.Text}
	if n := len(history); n > 0 && history[n-1].Role == agent.RoleBot && len(history[n-1].Calls) == 0 {
		bot.Text = strings.TrimSpace(history[n-1].Text + "\n\n" + r.Text)
		history = history[:n-1]
	}
	var results []agent.Message
	for _, c := range r.Calls {
		bot.Calls = append(bot.Calls, agent.ToolCall{ID: c.ID, Name: c.Name, Args: json.RawMessage(c.Arguments)})
		results = append(results, agent.Message{Role: agent.RoleTool, CallID: c.ID, Text: r.result(c.ID)})
	}
	return append(append(history, bot), results...), note
}

// result is what the round's call id returned, or that it never ran.
func (r Round) result(id string) string {
	if res, ok := r.Results[id]; ok {
		return res
	}
	return NotRun
}

// asText writes the round out as one note from the app: what the bot said,
// then each call and its result.
func asText(r Round) string {
	var b strings.Builder
	b.WriteString("(hearth) Before you stopped to wait for the owner")
	if r.Text != "" {
		fmt.Fprintf(&b, ", you wrote:\n\n%s\n\nThen you", r.Text)
	} else {
		b.WriteString(", you")
	}
	b.WriteString(" called these tools:\n")
	for _, c := range r.Calls {
		fmt.Fprintf(&b, "\n- %s with %s. The result: %s", c.Name, c.Arguments, r.result(c.ID))
	}
	return b.String()
}
