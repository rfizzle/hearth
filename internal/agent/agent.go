package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	coreagent "github.com/rfizzle/agentcore/agent"
	"github.com/rfizzle/agentcore/retry"
	"github.com/rfizzle/llm"
)

// The three ways a turn is stopped short. They are distinct so the caller
// can tell a run cut off by a cap from one the person stopped.
// See docs/model.md#4-termination-guarantees.
var (
	// ErrRoundCap ends a turn that used up its tool rounds without answering.
	ErrRoundCap = errors.New("tool round cap reached")
	// ErrWallCap ends a turn that ran past its wall-time cap.
	ErrWallCap = errors.New("wall-time cap reached")
	// ErrCancelled ends a turn whose context was cancelled.
	ErrCancelled = errors.New("turn cancelled")
)

// Tool is one tool the model may call, in plain Go: the name, what it is
// for, and its JSON-schema parameters.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Stream opens one completion over the messages and the tools on offer. The
// context ends when the request is abandoned. choice is the loop's word on
// calling a tool ("auto" or "none").
type Stream func(ctx context.Context, msgs []llm.Message, tools []llm.Tool, choice string) (<-chan llm.StreamEvent, error)

// Result is what a tool call produced: Text goes back to the model, Files
// (absolute paths) go to the sink for the person.
type Result struct {
	Text  string
	Files []string
}

// Execute runs one tool call. The context ends when the turn is stopped.
type Execute func(ctx context.Context, name string, args json.RawMessage) (Result, error)

// Sink is told what the turn does as it does it, so the caller can persist
// and show it. Calls never overlap.
type Sink interface {
	// Text is a streamed fragment of the assistant's reply.
	Text(delta string)
	// ToolCall is a call the model asked for, before it runs.
	ToolCall(id, name string, args json.RawMessage)
	// ToolResult is the outcome of one call; err is the executor's own.
	ToolResult(id string, r Result, err error, d time.Duration)
	// Round is told when the n-th tool round begins.
	Round(n int)
	// Retry is told before the turn waits to ask the provider again.
	Retry(attempt, max int, wait time.Duration)
}

// Input is everything one turn needs.
type Input struct {
	System  string    // the framing and the job description
	History []Message // earlier turns, oldest first
	Prompt  string    // the message being answered
	Tools   []Tool
	Stream  Stream
	Execute Execute
	// MaxRounds caps tool rounds; zero takes agentcore's own default, so a
	// caller that means a cap always passes one.
	MaxRounds int
	// MaxWall caps the turn's duration; zero means no cap.
	MaxWall time.Duration
	// Scrub is applied to every message before it is sent and to each tool
	// result before the sink sees it. Nil scrubs nothing.
	Scrub Scrub
	Sink  Sink // nil discards
}

// Outcome is how far a turn got. On a stopped or failed turn Text is the
// reply streamed since the last tool round, so the caller keeps what the
// person already saw.
type Outcome struct {
	Text   string
	Rounds int
}

// Run drives one turn to its end: stream, tool rounds, provider retry, the
// caps. It returns ErrRoundCap, ErrWallCap or ErrCancelled when stopped, and
// the provider's own classified failure when the model could not be reached;
// Explain words either for a person.
func Run(ctx context.Context, in Input) (Outcome, error) {
	if ctx.Err() != nil {
		return Outcome{}, ErrCancelled
	}
	// Requests end through Interrupt, which marks the turn stopped before it
	// aborts the stream; a request that ended on the caller's context first
	// would read as a short reply, not a stop.
	runCtx, cancelRun := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRun()

	t := newTurn(ctx, runCtx, in)
	defer t.watch()()

	text, err := t.h.Run(in.Prompt)
	return t.outcome(text, err)
}

// turn is the state one Run shares among the loop's callbacks.
type turn struct {
	ctx, runCtx context.Context
	in          Input
	tools       []llm.Tool
	a           *coreagent.Agent
	h           *coreagent.Headless
	side        *sideband // what an executor produced past the loop's text
	wall        atomic.Bool

	mu        sync.Mutex      // one sink call at a time
	cur       strings.Builder // reply streamed since the last tool round
	lastRound int
}

// newTurn builds the loop from the input and wires its callbacks.
func newTurn(ctx, runCtx context.Context, in Input) *turn {
	t := &turn{ctx: ctx, runCtx: runCtx, in: in, side: &sideband{}}
	t.tools = make([]llm.Tool, len(in.Tools))
	for i, tl := range in.Tools {
		t.tools[i] = llm.Tool{Name: tl.Name, Description: tl.Description, Parameters: tl.Parameters}
	}
	initial := make([]llm.Message, 0, len(in.History)+1)
	if in.System != "" {
		initial = append(initial, llm.Message{Role: llm.RoleSystem, Content: in.System})
	}
	for _, m := range in.History {
		initial = append(initial, llm.Message{Role: llmRole(m.Role), Content: m.Text})
	}

	t.a = coreagent.New(initial, t.stream)
	t.a.SetMaxRounds(in.MaxRounds)
	if in.Scrub != nil {
		t.a.SetScrub(in.Scrub.core)
	}
	t.a.SetExecutor(t.execute)
	t.h = &coreagent.Headless{
		Agent:        t.a,
		OnText:       t.onText,
		OnToolCall:   t.onToolCall,
		OnToolResult: t.onToolResult,
		OnRetry:      t.onRetry,
	}
	return t
}

// stream opens one request. A context that ended before the first request
// is honoured here, since the loop's own start would reset an earlier stop.
func (t *turn) stream(msgs []llm.Message, choice string) (<-chan llm.StreamEvent, context.CancelFunc, error) {
	reqCtx, cancel := context.WithCancel(t.runCtx)
	if t.ctx.Err() != nil {
		t.h.Interrupt()
	}
	ch, err := t.in.Stream(reqCtx, msgs, t.tools, choice)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return ch, cancel, nil
}

// execute runs a call. The loop takes a tool's result as text alone, so the
// files and the error are left in the side channel for onToolResult.
func (t *turn) execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	var r Result
	var err error
	if t.in.Execute == nil {
		err = errors.New("no tools are available")
	} else {
		r, err = t.in.Execute(ctx, name, args)
	}
	t.side.put(name, string(args), r, err)
	return r.Text, err
}

func (t *turn) tell(f func(Sink)) {
	if t.in.Sink == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f(t.in.Sink)
}

func (t *turn) onText(s string) {
	t.cur.WriteString(s)
	t.tell(func(k Sink) { k.Text(s) })
}

func (t *turn) onToolCall(tc llm.ToolCall) {
	t.cur.Reset()
	if n := t.a.Rounds(); n != t.lastRound {
		t.lastRound = n
		t.tell(func(k Sink) { k.Round(n) })
	}
	t.tell(func(k Sink) { k.ToolCall(tc.ID, tc.Name, json.RawMessage(tc.Arguments)) })
}

func (t *turn) onToolResult(r coreagent.ToolResult) {
	res, err := t.side.take(r.Call.Name, r.Call.Arguments)
	// A call stopped before it ran has no entry; the loop's own text for it
	// stands.
	if res.Text == "" && err == nil {
		res.Text = r.Result
	}
	res.Text = scrubText(t.in.Scrub, res.Text)
	t.tell(func(k Sink) { k.ToolResult(r.Call.ID, res, err, r.Duration) })
}

func (t *turn) onRetry(n retry.RetryNotice) {
	t.cur.Reset()
	t.tell(func(k Sink) { k.Retry(n.Attempt, n.Max, n.Wait) })
}

// watch stops the turn when the wall-time cap passes or the caller's context
// ends. The returned function ends the watch.
func (t *turn) watch() func() {
	var timer *time.Timer
	if t.in.MaxWall > 0 {
		timer = time.AfterFunc(t.in.MaxWall, func() {
			t.wall.Store(true)
			t.h.Interrupt()
		})
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-t.ctx.Done():
			t.h.Interrupt()
		case <-done:
		}
	}()
	return func() {
		if timer != nil {
			timer.Stop()
		}
		close(done)
	}
}

// outcome maps the loop's end to hearth's: the reply, the rounds used, and
// which way the turn was stopped. A turn that did not finish keeps the text
// streamed since its last tool round.
func (t *turn) outcome(text string, err error) (Outcome, error) {
	out := Outcome{Text: text, Rounds: t.a.Rounds()}
	if err == nil {
		return out, nil
	}
	out.Text = t.cur.String()
	switch {
	case errors.Is(err, coreagent.ErrRoundCap):
		return out, fmt.Errorf("%w after %d rounds", ErrRoundCap, out.Rounds)
	case errors.Is(err, coreagent.ErrInterrupted) && t.wall.Load():
		return out, ErrWallCap
	case errors.Is(err, coreagent.ErrInterrupted):
		return out, ErrCancelled
	}
	return out, err
}

func scrubText(scrub Scrub, s string) string {
	if scrub == nil || s == "" {
		return s
	}
	return scrub(Message{Role: RoleTool, Text: s}).Text
}

// sideband carries what an executor produced past the loop's text, matched
// to a result by the call's name and arguments. Identical calls queue in
// order; they ask for the same thing, so which result takes which entry
// does not change what either reads.
type sideband struct {
	mu sync.Mutex
	q  map[string][]entry
}

type entry struct {
	res Result
	err error
}

func (s *sideband) put(name, args string, r Result, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.q == nil {
		s.q = map[string][]entry{}
	}
	k := name + "\x00" + args
	s.q[k] = append(s.q[k], entry{r, err})
}

func (s *sideband) take(name, args string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := name + "\x00" + args
	es := s.q[k]
	if len(es) == 0 {
		return Result{}, nil
	}
	s.q[k] = es[1:]
	return es[0].res, es[0].err
}
