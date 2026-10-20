package assembly

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org/memory"
)

// wall ends a side run that is taking too long.
const wall = 90 * time.Second

// Side is the model a side run asks, once and with no tools: the stream,
// the provider and model it reports its usage under, the scrub every
// message passes, and where each request's usage goes. A nil Scrub scrubs
// nothing and a nil Usage reports nothing.
// See docs/model.md#usage.
type Side struct {
	Stream agent.Stream
	Model  agent.Model
	Scrub  agent.Scrub
	Usage  func(provider, model string, u agent.Usage)
}

// ask runs one round with system and prompt and returns the reply's text.
func (s Side) ask(ctx context.Context, system, prompt string) (string, error) {
	var sink agent.Sink
	if s.Usage != nil {
		sink = usageSink(s.Usage)
	}
	out, err := agent.Run(ctx, agent.Input{
		System: system, Prompt: prompt,
		Stream: s.Stream, Model: s.Model, Scrub: s.Scrub, Sink: sink,
		MaxRounds: 1, MaxWall: wall,
	})
	return out.Text, err
}

// usageSink hears a side run's usage and nothing else: no message of a side
// run is stored.
type usageSink func(provider, model string, u agent.Usage)

func (f usageSink) Usage(provider, model string, u agent.Usage)         { f(provider, model, u) }
func (usageSink) Text(string)                                           {}
func (usageSink) ToolCall(string, string, json.RawMessage)              {}
func (usageSink) ToolResult(string, agent.Result, error, time.Duration) {}
func (usageSink) Round(int)                                             {}
func (usageSink) Retry(int, int, time.Duration)                         {}

// TidyDue reports whether a bot with notes notes is due a tidy: at the cap
// or over it, after a run that ended done.
// See docs/model.md#7-memory-writes.
func TidyDue(notes int) bool { return notes >= memory.Cap }

// Changed is the notes of was that are not in now, by id, and the notes of
// now that were not in was: what a tidy removed and added.
func Changed(was, now []memory.Note) (gone, added []memory.Note) {
	ids := func(notes []memory.Note) map[string]bool {
		m := make(map[string]bool, len(notes))
		for _, n := range notes {
			m[n.ID] = true
		}
		return m
	}
	before, after := ids(was), ids(now)
	for _, n := range was {
		if !after[n.ID] {
			gone = append(gone, n)
		}
	}
	for _, n := range now {
		if !before[n.ID] {
			added = append(added, n)
		}
	}
	return gone, added
}
