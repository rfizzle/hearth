package record

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools"
)

// Meter is what a sink reports each model request to. It is nil when
// nothing is metered.
type Meter interface {
	Record(ctx context.Context, r usage.Record) error
}

// Host is what a sink reaches the service through: the store it writes, the
// bus it publishes on, the usage meter, the log and the clock, and Release,
// which frees the run's conversation for the next message.
type Host struct {
	Store *store.DB
	Bus   *bus.Bus
	Meter Meter
	Log   *log.Logger
	Now   func() time.Time
	// Release is called once the run's final state is stored, before the
	// bus hears it ended.
	Release func(conv, run string)
}

// Sink stores and publishes what one run's turn does as it does it. The bot's
// reply is one text message per stretch of text: a tool call closes the
// stretch before it, and the next token opens another.
type Sink struct {
	h       Host
	ctx     context.Context // for writes; a stop does not cancel it
	botID   string
	botName string

	mu       sync.Mutex
	run      org.Run
	text     *org.Message // the message streaming now, nil between stretches
	buf      strings.Builder
	calls    map[string]org.ToolCall // by call id, for the result that answers it
	describe map[string]func(json.RawMessage) string
	err      error // the first write that failed
	stop     context.CancelFunc
	panicked any         // the first panic, nil if none
	opened   *inbox.Item // the item a parked run opened, published by finish
	disp     tools.Dispatcher
}

var _ agent.Sink = (*Sink)(nil)

// New is a sink for run r of the bot botID, named botName. Its writes use
// ctx, which a stop must not cancel.
func New(h Host, ctx context.Context, r org.Run, botID, botName string) *Sink {
	return &Sink{h: h, ctx: ctx, run: r, botID: botID, botName: botName,
		calls: map[string]org.ToolCall{}, describe: map[string]func(json.RawMessage) string{}}
}

// Start marks the run working from now and publishes it.
func (k *Sink) Start() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.run.State = org.RunWorking
	k.run.StartedAt = k.h.Now()
	k.saveRunLocked()
}

// SetStop is what Crash calls to stop the turn.
func (k *Sink) SetStop(stop context.CancelFunc) { k.stop = stop }

// Equip gives the sink the dispatcher that checks the run's calls, and the
// describers of the tools it offers, for the summaries of calls it records.
func (k *Sink) Equip(disp tools.Dispatcher) {
	k.disp = disp
	for _, d := range disp.Tools() {
		k.describe[d.Name] = d.Describe
	}
}

// Crash records the first panic of the run, logs it with the stack of the
// goroutine it happened on, and stops the turn. Call it from the deferred
// recover.
func (k *Sink) Crash(p any) {
	k.h.Log.Printf("orchestrate: run %s panicked: %v\n%s", k.run.ID, p, debug.Stack())
	k.mu.Lock()
	if k.panicked == nil {
		k.panicked = p
	}
	k.mu.Unlock()
	if k.stop != nil {
		k.stop()
	}
}

// Crashed is the first panic of the run, or nil.
func (k *Sink) Crashed() any {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.panicked
}

// Text opens a streaming message on the first token of a stretch and
// publishes each token as a delta; the tokens are stored when the stretch
// ends.
func (k *Sink) Text(delta string) {
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
	k.h.Bus.Publish(bus.MessageDelta{
		ConversationID: k.run.ConversationID, MessageID: k.text.ID, RunID: k.run.ID, Text: delta,
	})
}

// Round records the n-th tool round as the run's step count.
func (k *Sink) Round(n int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.closeText(k.buf.String())
	k.run.Steps = n
	k.saveRunLocked()
}

// ToolCall stores the call with its tool's plain-words summary. A call the
// dispatcher places is summarised as the card is, with the path it
// resolved, so the row never shows the bot's paraphrase of where it lands;
// one it refuses is summarised from its own arguments.
// See docs/model.md#8-approvals.
func (k *Sink) ToolCall(id, name string, args json.RawMessage) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.closeText(k.buf.String())
	tc := org.ToolCall{ID: id, Name: name, Arguments: string(args)}
	if c, err := k.disp.Check(name, args); err == nil {
		tc.Summary = c.Describe()
	} else if d := k.describe[name]; d != nil {
		tc.Summary = d(args)
	}
	k.calls[id] = tc
	k.append(org.Message{Kind: org.MessageToolCall, ToolCalls: []org.ToolCall{tc}})
}

// ToolResult stores what the call returned, the call it answers and the
// files it wrote.
func (k *Sink) ToolResult(id string, r agent.Result, err error, _ time.Duration) {
	if errors.Is(err, agent.ErrParked) {
		// The call has not run. Its result is written when the owner
		// decides, and a call the run did not park on is owed none.
		return
	}
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

// Usage reports one request to the meter, with the run and the bot it was
// made for. A request the meter cannot store is logged: the run goes on, and
// the figure is short by that one.
// See docs/model.md#usage.
func (k *Sink) Usage(provider, model string, u agent.Usage) {
	if k.h.Meter == nil {
		return
	}
	err := k.h.Meter.Record(k.ctx, usage.Record{
		RunID: k.run.ID, BotID: k.botID, ConversationID: k.run.ConversationID,
		Provider: provider, Model: model,
		Prompt: int64(u.Prompt), Completion: int64(u.Completion), Cached: int64(u.Cached), Created: int64(u.Created),
		At: k.h.Now(),
	})
	if err != nil {
		k.h.Log.Printf("orchestrate: run %s: could not record its usage: %v", k.run.ID, err)
	}
}

// Retry empties the streaming message: the attempt that failed streamed
// tokens the next attempt starts over from.
func (k *Sink) Retry(int, int, time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.text == nil {
		return
	}
	k.buf.Reset()
	k.text.Content = ""
	k.update(*k.text)
}

// Finish closes what the turn left open and ends the run: done, cancelled
// with what streamed kept, waiting on the owner with the request in the
// thread and the Inbox, or failed with the reason in plain words in the run
// and in the thread. A provider failure, a cap or a run that could not start
// all end it failed; a provider failure also leaves an item in the Inbox.
// See docs/model.md#9-failure-handling.
func (k *Sink) Finish(out agent.Outcome, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err == nil && k.err != nil {
		err = k.err
	}
	if errors.Is(err, agent.ErrParked) {
		k.endText()
		if err = k.park(out.Parked); err == nil {
			k.run.State = org.RunWaitingOwner
		}
	}
	switch {
	case k.run.State == org.RunWaitingOwner:
	case err == nil:
		if k.text == nil && out.Text != "" {
			k.append(org.Message{Kind: org.MessageText, Content: out.Text})
		}
		k.closeText(out.Text)
		k.run.State = org.RunDone
	case errors.Is(err, agent.ErrCancelled):
		k.endText()
		k.run.State = org.RunCancelled
	default:
		k.endText()
		k.run.State = org.RunFailed
		k.run.Failure = agent.Explain(err)
		// The SYSTEM row carries the bot and the run, like the bot's own
		// messages: the owner did not write it, and it belongs to the run
		// it explains.
		k.append(org.Message{Kind: org.MessageSystem, Content: Sentence(k.run.Failure)})
		k.openFailure(err)
	}
	if k.run.State != org.RunWaitingOwner {
		k.run.EndedAt = k.h.Now()
	}
	if err := k.h.Store.UpdateRun(k.ctx, k.run); err != nil {
		k.fail(err)
	}
	// Free the conversation before saying the run ended, so a message sent
	// on that news is not refused as busy. A waiting run's conversation is
	// held by its open item instead.
	k.h.Release(k.run.ConversationID, k.run.ID)
	k.h.Bus.Publish(bus.RunChanged{Run: k.run})
	if k.opened != nil {
		k.h.Bus.Publish(bus.InboxChanged{Item: *k.opened})
	}
}

func (k *Sink) saveRunLocked() {
	if err := k.h.Store.UpdateRun(k.ctx, k.run); err != nil {
		k.fail(err)
	}
	k.h.Bus.Publish(bus.RunChanged{Run: k.run})
}

// endText closes the streaming message a run that did not finish leaves
// open, keeping what it has; one that has streamed nothing is deleted rather
// than left as an empty bubble.
func (k *Sink) endText() {
	if k.text == nil || k.buf.Len() > 0 {
		k.closeText(k.buf.String())
		return
	}
	if err := k.h.Store.DeleteMessage(k.ctx, k.text.ID); err != nil {
		k.fail(err)
	} else {
		k.h.Bus.Publish(bus.MessageDeleted{ConversationID: k.run.ConversationID, MessageID: k.text.ID})
	}
	k.text = nil
}

// closeText ends the streaming message, if one is open, with content.
func (k *Sink) closeText(content string) {
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
func (k *Sink) append(m org.Message) (org.Message, bool) {
	m.ConversationID = k.run.ConversationID
	m.AuthorBotID = k.botID
	m.RunID = k.run.ID
	stored, err := k.h.Store.AppendMessage(k.ctx, m)
	if err != nil {
		k.fail(err)
		return org.Message{}, false
	}
	k.h.Bus.Publish(bus.MessageUpserted{Message: stored})
	return stored, true
}

// update stores m's new content and publishes it.
func (k *Sink) update(m org.Message) {
	if err := k.h.Store.UpdateMessage(k.ctx, m); err != nil {
		k.fail(err)
		return
	}
	k.h.Bus.Publish(bus.MessageUpserted{Message: m})
}

func (k *Sink) fail(err error) {
	if k.err == nil {
		k.err = err
	}
}

// Sentence is s with a capital first letter and a full stop.
func Sentence(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[n:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}
