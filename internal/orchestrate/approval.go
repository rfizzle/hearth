package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/orchestrate/park"
	"github.com/rfizzle/hearth/internal/orchestrate/record"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools"
)

// ErrClosed refuses a decision on an item already dealt with, or whose run
// no longer waits on it.
var ErrClosed = errors.New("that has already been dealt with")

// ErrMismatch refuses an answer the item does not take: a decision on a
// question, an answer to an approval, an acknowledgement of either, or a
// decision that is none of the four.
var ErrMismatch = errors.New("that item does not take this answer")

// parked is an open item and the run waiting on it, with the conversation
// claimed for the run that resumes it.
type parked struct {
	item inbox.Item
	run  org.Run
	bot  org.Bot
	call org.ToolCall
}

// outcome is what the parked call came to: the text the bot reads as its
// result, and the files it wrote.
type outcome struct {
	text  string
	files []org.FileRef
}

// Decide settles the approval itemID with the owner's decision, runs the
// call it held when the decision allows it, records both in the thread,
// closes the item and starts the run that carries on, returning the closed
// item and that run. Allow for this task stores an allowance for the
// conversation; always allow raises the grant and is inbox.ErrNoStanding
// for a tool that has no standing; deny gives the bot the owner's reason.
// See docs/model.md#8-approvals.
func (s *Service) Decide(ctx context.Context, itemID string, d inbox.Decision, reason string) (inbox.Item, org.Run, error) {
	if d < inbox.AllowOnce || d > inbox.Deny {
		return inbox.Item{}, org.Run{}, fmt.Errorf("decision %d: %w", d, ErrMismatch)
	}
	p, err := s.claim(ctx, itemID, inbox.Approval)
	if err != nil {
		return inbox.Item{}, org.Run{}, err
	}
	ctx = context.WithoutCancel(ctx)
	said, call, err := s.decide(ctx, p, d, reason)
	if err != nil {
		s.release(p.item.ConversationID, "")
		return inbox.Item{}, org.Run{}, err
	}
	owner := org.Message{Kind: org.MessageApprovalDecision, Content: said}
	return s.settle(ctx, p, owner, call, d, "", said)
}

// Acknowledge closes the failure or budget item itemID. No run waits on one.
func (s *Service) Acknowledge(ctx context.Context, itemID string) (inbox.Item, error) {
	it, err := s.d.Store.InboxItem(ctx, itemID)
	if err != nil {
		return inbox.Item{}, err
	}
	if it.Kind != inbox.Failure && it.Kind != inbox.Budget {
		return inbox.Item{}, fmt.Errorf("item %s: %w", itemID, ErrMismatch)
	}
	if err := s.d.Store.CloseInboxItem(ctx, itemID, 0, "", "Acknowledged"); err != nil {
		if errors.Is(err, store.ErrItemClosed) {
			return inbox.Item{}, fmt.Errorf("item %s: %w", itemID, ErrClosed)
		}
		return inbox.Item{}, err
	}
	return s.closed(ctx, itemID)
}

// claim loads the open item itemID of kind and the run waiting on it, and
// claims the conversation so nothing else starts there while the item is
// settled. The item and the run are read again once it is claimed, so of two
// decisions on one item only the first settles it.
func (s *Service) claim(ctx context.Context, itemID string, kind inbox.Kind) (parked, error) {
	it, err := s.d.Store.InboxItem(ctx, itemID)
	if err != nil {
		return parked{}, err
	}
	if err := s.reserve(it.ConversationID); err != nil {
		return parked{}, err
	}
	p, err := s.load(ctx, itemID, kind)
	if err != nil {
		s.release(it.ConversationID, "")
	}
	return p, err
}

// load reads what claim checks.
func (s *Service) load(ctx context.Context, itemID string, kind inbox.Kind) (parked, error) {
	var p parked
	var err error
	if p.item, err = s.d.Store.InboxItem(ctx, itemID); err != nil {
		return parked{}, err
	}
	if p.item.Kind != kind || p.item.Call == nil {
		return parked{}, fmt.Errorf("item %s: %w", itemID, ErrMismatch)
	}
	if p.item.State != inbox.Open {
		return parked{}, fmt.Errorf("item %s: %w", itemID, ErrClosed)
	}
	if p.run, err = s.d.Store.Run(ctx, p.item.RunID); err != nil {
		return parked{}, err
	}
	if p.run.State != org.RunWaitingOwner {
		return parked{}, fmt.Errorf("run %s: %w", p.run.ID, ErrClosed)
	}
	if p.bot, err = s.d.Store.Bot(ctx, p.item.BotID); err != nil {
		return parked{}, err
	}
	p.call = *p.item.Call
	return p, nil
}

// decide applies d to the parked call: it stores what the decision leaves
// behind and returns the words the thread records for it, and what running
// the call comes to, for settle to call once the item is closed.
func (s *Service) decide(ctx context.Context, p parked, d inbox.Decision, reason string) (string, func() outcome, error) {
	if d == inbox.Deny {
		return park.Declined(reason), func() outcome { return outcome{text: park.Denial(reason)} }, nil
	}
	disp, err := s.dispatcher(ctx, p.bot, p.item.ConversationID, "")
	if err != nil {
		return "", nil, err
	}
	args := json.RawMessage(p.call.Arguments)
	// A call the grants now refuse stores nothing; running it gives the bot
	// the refusal.
	folder := ""
	if c, err := disp.Check(p.call.Name, args); err == nil {
		if disp, err = s.allow(ctx, p, disp, c, d); err != nil {
			return "", nil, err
		}
		if c.Granted != "" {
			folder = filepath.Base(c.Granted)
		}
	}
	return park.Allowed(d, p.bot.Name, folder), func() outcome { return s.runApproved(ctx, disp, p.call) }, nil
}

// allow stores what an allow decision leaves behind and returns the
// dispatcher with it: an allowance for the conversation, or the bot's grants
// raised for good. Allow once stores nothing.
func (s *Service) allow(ctx context.Context, p parked, disp tools.Dispatcher, c tools.Checked, d inbox.Decision) (tools.Dispatcher, error) {
	if d == inbox.AlwaysAllow && !c.Def.Standing() {
		return disp, inbox.ErrNoStanding
	}
	_, allowance, err := inbox.Apply(d, disp.Grants, c.Def.Capability, c.Granted)
	if err != nil {
		return disp, err
	}
	switch d {
	case inbox.AllowForTask:
		allowance.ConversationID = p.item.ConversationID
		if err := s.d.Store.AddAllowance(ctx, *allowance); err != nil {
			return disp, err
		}
		disp.Allowances = append(disp.Allowances, *allowance)
	case inbox.AlwaysAllow:
		grants, err := s.d.Store.GrantsFor(ctx, p.bot.ID)
		if err != nil {
			return disp, err
		}
		if err := s.d.Store.ReplaceGrants(ctx, p.bot.ID, park.Raise(grants, c.Def.Capability, c.Granted)); err != nil {
			return disp, err
		}
		if disp.Grants, err = s.d.Store.GrantsFor(ctx, p.bot.ID); err != nil {
			return disp, err
		}
		s.d.Bus.Publish(GrantsChanged{BotID: p.bot.ID, Grants: disp.Grants})
	}
	return disp, nil
}

// runApproved runs the call the owner allowed, outside any model loop,
// through the dispatcher's approved door: the grants and the path are still
// checked. A refusal, an error or a panic is the text the bot reads.
func (s *Service) runApproved(ctx context.Context, disp tools.Dispatcher, call org.ToolCall) (out outcome) {
	ctx, cancel := context.WithTimeout(ctx, s.d.Caps.Wall)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			s.d.Log.Printf("orchestrate: approved call %s panicked: %v", call.Name, p)
			out = outcome{text: call.Name + " stopped unexpectedly"}
		}
	}()
	res, err := disp.ExecuteApproved(ctx, call.Name, json.RawMessage(call.Arguments))
	out.text = res.Text
	if err != nil && out.text == "" {
		out.text = err.Error()
	}
	if s.d.Scrub != nil {
		out.text = s.d.Scrub(agent.Message{Role: agent.RoleTool, Text: out.text}).Text
	}
	for _, f := range res.Files {
		out.files = append(out.files, org.FileRef{Path: f.Path, Name: filepath.Base(f.Path)})
	}
	return out
}

// settle closes the item with the decision, answer and outcome, then runs
// call and records the owner's message and its result in the thread, ends
// the parked run done, and starts the run that carries on from it on fresh
// context. The item closes first so a call runs at most once: a failure
// after it leaves no open item for a second decision to run it again. The
// conversation is claimed on entry and stays held by the new run.
// See docs/model.md#2-a-run.
func (s *Service) settle(ctx context.Context, p parked, owner org.Message, call func() outcome, d inbox.Decision, answer, said string) (inbox.Item, org.Run, error) {
	conv := p.item.ConversationID
	fail := func(err error) (inbox.Item, org.Run, error) {
		s.release(conv, "")
		return inbox.Item{}, org.Run{}, err
	}
	if err := s.d.Store.CloseInboxItem(ctx, p.item.ID, d, answer, said); err != nil {
		return fail(err)
	}
	item, err := s.closed(ctx, p.item.ID)
	if err != nil {
		return fail(err)
	}
	if err := s.record(ctx, p, owner, call()); err != nil {
		s.abandon(ctx, p)
		return fail(err)
	}
	note := org.Message{ConversationID: conv, Content: assembly.Note}
	r, runCtx, err := s.start(ctx, p.bot, note)
	if err != nil {
		return fail(err)
	}
	go s.run(runCtx, p.bot, r, ask{msg: note, parked: p.run.ID})
	return item, r, nil
}

// record writes the owner's message and the call's result to the thread
// and ends the parked run done: it did what it could, and the new run
// continues it.
func (s *Service) record(ctx context.Context, p parked, owner org.Message, out outcome) error {
	if s.fault != nil {
		if err := s.fault(); err != nil {
			return err
		}
	}
	owner.ConversationID = p.item.ConversationID
	result := org.Message{
		ConversationID: p.item.ConversationID, AuthorBotID: p.bot.ID, RunID: p.run.ID, Kind: org.MessageToolResult,
		Content: out.text, ToolCallID: p.call.ID, ToolCalls: []org.ToolCall{p.call}, Files: out.files,
	}
	for _, m := range []org.Message{owner, result} {
		stored, err := s.d.Store.AppendMessage(ctx, m)
		if err != nil {
			return err
		}
		s.d.Bus.Publish(MessageUpserted{Message: stored})
	}
	p.run.State, p.run.EndedAt = org.RunDone, s.d.Now()
	if err := s.d.Store.UpdateRun(ctx, p.run); err != nil {
		return err
	}
	s.d.Bus.Publish(RunChanged{Run: p.run})
	return nil
}

// abandonReason is why a parked run failed after the owner's decision.
const abandonReason = "your decision was recorded, but the conversation could not be updated with what came of it"

// abandon ends the parked run failed when the thread could not record what
// a decision came to, with the reason in the thread, so it does not wait on
// an item that is already closed and the owner is offered Retry. A failure
// here as well is logged.
// See docs/model.md#9-failure-handling.
func (s *Service) abandon(ctx context.Context, p parked) {
	p.run.State, p.run.Failure, p.run.EndedAt = org.RunFailed, abandonReason, s.d.Now()
	if err := s.d.Store.UpdateRun(ctx, p.run); err != nil {
		s.d.Log.Printf("orchestrate: run %s could not be ended after a decision: %v", p.run.ID, err)
		return
	}
	s.d.Bus.Publish(RunChanged{Run: p.run})
	m, err := s.d.Store.AppendMessage(ctx, org.Message{ConversationID: p.item.ConversationID, AuthorBotID: p.bot.ID,
		RunID: p.run.ID, Kind: org.MessageSystem, Content: record.Sentence(abandonReason)})
	if err != nil {
		s.d.Log.Printf("orchestrate: run %s: %v", p.run.ID, err)
		return
	}
	s.d.Bus.Publish(MessageUpserted{Message: m})
}

// closed reads the item itemID after it closed and publishes it.
func (s *Service) closed(ctx context.Context, itemID string) (inbox.Item, error) {
	it, err := s.d.Store.InboxItem(ctx, itemID)
	if err != nil {
		return inbox.Item{}, err
	}
	s.d.Bus.Publish(InboxChanged{Item: it})
	return it, nil
}

// cancelWaiting ends the run runID cancelled if it waits on the owner, and
// closes the item it waits on, recording both. It reports whether it did.
// See docs/model.md#8-approvals.
func (s *Service) cancelWaiting(ctx context.Context, runID string) bool {
	r, err := s.d.Store.Run(ctx, runID)
	if err != nil || r.State != org.RunWaitingOwner || s.reserve(r.ConversationID) != nil {
		return false
	}
	defer s.release(r.ConversationID, "")
	// Read again once claimed: a decision may have settled it first.
	if r, err = s.d.Store.Run(ctx, runID); err != nil || r.State != org.RunWaitingOwner {
		return false
	}
	if it, open, err := s.d.Store.OpenItemFor(ctx, r.ConversationID); err == nil && open && it.RunID == runID {
		if err := s.d.Store.CloseInboxItem(ctx, it.ID, 0, "", "Cancelled"); err == nil {
			_, _ = s.closed(ctx, it.ID)
		}
	}
	if m, err := s.d.Store.AppendMessage(ctx, org.Message{ConversationID: r.ConversationID, Kind: org.MessageApprovalDecision, Content: "Cancelled"}); err == nil {
		s.d.Bus.Publish(MessageUpserted{Message: m})
	}
	r.State, r.EndedAt = org.RunCancelled, s.d.Now()
	if err := s.d.Store.UpdateRun(ctx, r); err != nil {
		return false
	}
	s.d.Bus.Publish(RunChanged{Run: r})
	return true
}
