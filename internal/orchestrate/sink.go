package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
)

// sink stores and publishes what one run's turn does as it does it. The bot's
// reply is one text message per stretch of text: a tool call closes the
// stretch before it, and the next token opens another.
type sink struct {
	s     *Service
	ctx   context.Context // for writes; a stop does not cancel it
	botID string

	mu       sync.Mutex
	run      org.Run
	text     *org.Message // the message streaming now, nil between stretches
	buf      strings.Builder
	calls    map[string]org.ToolCall // by call id, for the result that answers it
	describe map[string]func(json.RawMessage) string
	err      error // the first write that failed
}

var _ agent.Sink = (*sink)(nil)

// Text opens a streaming message on the first token of a stretch and
// publishes each token as a delta; the tokens are stored when the stretch
// ends.
func (k *sink) Text(delta string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.text == nil {
		m, ok := k.append(org.Message{Kind: org.MessageText, Streaming: true})
		if !ok {
			return
		}
		k.text = &m
		k.buf.Reset()
	}
	k.buf.WriteString(delta)
	k.s.d.Bus.Publish(MessageDelta{
		ConversationID: k.run.ConversationID, MessageID: k.text.ID, RunID: k.run.ID, Text: delta,
	})
}

// Round records the n-th tool round as the run's step count.
func (k *sink) Round(n int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.closeText(k.buf.String())
	k.run.Steps = n
	k.saveRunLocked()
}

// ToolCall stores the call with its tool's plain-words summary.
func (k *sink) ToolCall(id, name string, args json.RawMessage) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.closeText(k.buf.String())
	tc := org.ToolCall{ID: id, Name: name, Arguments: string(args)}
	if d := k.describe[name]; d != nil {
		tc.Summary = d(args)
	}
	k.calls[id] = tc
	k.append(org.Message{Kind: org.MessageToolCall, ToolCalls: []org.ToolCall{tc}})
}

// ToolResult stores what the call returned, the call it answers and the
// files it wrote.
func (k *sink) ToolResult(id string, r agent.Result, err error, _ time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	m := org.Message{Kind: org.MessageToolResult, Content: r.Text, ToolCallID: id}
	if err != nil && m.Content == "" {
		m.Content = err.Error()
	}
	if tc, ok := k.calls[id]; ok {
		m.ToolCalls = []org.ToolCall{tc}
	}
	for _, p := range r.Files {
		m.Files = append(m.Files, org.FileRef{Path: p, Name: filepath.Base(p)})
	}
	k.append(m)
}

// Retry empties the streaming message: the attempt that failed streamed
// tokens the next attempt starts over from.
func (k *sink) Retry(int, int, time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.text == nil {
		return
	}
	k.buf.Reset()
	k.text.Content = ""
	k.update(*k.text)
}

// finish closes what the turn left open and ends the run: done, cancelled
// with what streamed kept, or failed with the reason in plain words in the
// run and in the thread. A provider failure, a cap or a run that could not
// start all end it failed.
// See docs/model.md#9-failure-handling.
func (k *sink) finish(out agent.Outcome, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err == nil && k.err != nil {
		err = k.err
	}
	switch {
	case err == nil:
		if k.text == nil && out.Text != "" {
			k.append(org.Message{Kind: org.MessageText, Content: out.Text})
		}
		k.closeText(out.Text)
		k.run.State = org.RunDone
	case errors.Is(err, agent.ErrCancelled):
		k.closeText(k.buf.String())
		k.run.State = org.RunCancelled
	default:
		k.closeText(k.buf.String())
		k.run.State = org.RunFailed
		k.run.Failure = agent.Explain(err)
		// The SYSTEM row carries the bot and the run, like the bot's own
		// messages: the owner did not write it, and it belongs to the run
		// it explains.
		k.append(org.Message{Kind: org.MessageSystem, Content: sentence(k.run.Failure)})
	}
	k.run.EndedAt = k.s.d.Now()
	if err := k.s.d.Store.UpdateRun(k.ctx, k.run); err != nil {
		k.fail(err)
	}
	// Free the conversation before saying the run ended, so a message sent
	// on that news is not refused as busy.
	k.s.release(k.run.ConversationID, k.run.ID)
	k.s.d.Bus.Publish(RunChanged{Run: k.run})
}

// saveRun stores the run's state and publishes it.
func (k *sink) saveRun() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.saveRunLocked()
}

func (k *sink) saveRunLocked() {
	if err := k.s.d.Store.UpdateRun(k.ctx, k.run); err != nil {
		k.fail(err)
	}
	k.s.d.Bus.Publish(RunChanged{Run: k.run})
}

// closeText ends the streaming message, if one is open, with content.
func (k *sink) closeText(content string) {
	if k.text == nil {
		return
	}
	k.text.Content = content
	k.text.Streaming = false
	k.update(*k.text)
	k.text = nil
	k.buf.Reset()
}

// append stores m as the bot's in this run and publishes it.
func (k *sink) append(m org.Message) (org.Message, bool) {
	m.ConversationID = k.run.ConversationID
	m.AuthorBotID = k.botID
	m.RunID = k.run.ID
	stored, err := k.s.d.Store.AppendMessage(k.ctx, m)
	if err != nil {
		k.fail(err)
		return org.Message{}, false
	}
	k.s.d.Bus.Publish(MessageUpserted{Message: stored})
	return stored, true
}

// update stores m's new content and publishes it.
func (k *sink) update(m org.Message) {
	if err := k.s.d.Store.UpdateMessage(k.ctx, m); err != nil {
		k.fail(err)
		return
	}
	k.s.d.Bus.Publish(MessageUpserted{Message: m})
}

func (k *sink) fail(err error) {
	if k.err == nil {
		k.err = err
	}
}

// sentence is s with a capital first letter and a full stop.
func sentence(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[n:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}
