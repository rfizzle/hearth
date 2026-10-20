package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// calls is a model reply that asks for one tool call.
func calls(id, name, args string) step {
	return func(context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent, 1)
		ch <- llm.StreamEvent{Done: true, ToolCalls: []llm.ToolCall{{ID: id, Name: name, Arguments: args}}}
		close(ch)
		return ch
	}
}

// broken is a model reply that fails the way a signed-out provider does.
func broken() step {
	return func(context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent, 1)
		ch <- llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}}
		close(ch)
		return ch
	}
}

// hireDana names the org, hires Dana and puts brief.md in her folder.
func hireDana(t *testing.T, c clients, r *rig) *hearthv1.Bot {
	t.Helper()
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	bot := unary(t, c.bot.CreateBot, &hearthv1.CreateBotRequest{Name: "Dana", JobDescription: "Help.", Tier: hearthv1.Tier_TIER_FAST}).GetBot()
	if err := os.MkdirAll(bot.GetFolderPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bot.GetFolderPath(), "brief.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bot
}

func runIn(state hearthv1.RunState) func(*hearthv1.SubscribeResponse) bool {
	return func(m *hearthv1.SubscribeResponse) bool { return m.GetRunChanged().GetRun().GetState() == state }
}

func inboxOpened(m *hearthv1.SubscribeResponse) bool {
	return m.GetInboxChanged().GetItem().GetState() == hearthv1.InboxState_INBOX_STATE_OPEN
}

func indexOf(evs []*hearthv1.SubscribeResponse, is func(*hearthv1.SubscribeResponse) bool) int {
	for i, e := range evs {
		if is(e) {
			return i
		}
	}
	return -1
}

// checkApprovalFlow is a bot asking to delete a file and the owner deciding:
// the park reaches the stream, a decision the core refuses leaves the item
// open, one it allows closes it and runs the call, and the same item cannot
// be decided twice. The scripted model asks to delete brief.md, then speaks.
// The in-memory and the loopback tests both run it.
func checkApprovalFlow(t *testing.T, c clients, r *rig) {
	t.Helper()
	ctx := t.Context()
	bot := hireDana(t, c, r)
	brief := filepath.Join(bot.GetFolderPath(), "brief.md")
	f := follow(t, c, r.bus)

	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "tidy up"})
	evs := f.until(inboxOpened)
	waiting := indexOf(evs, runIn(hearthv1.RunState_RUN_STATE_WAITING_OWNER))
	asked := indexOf(evs, func(m *hearthv1.SubscribeResponse) bool {
		return m.GetMessageUpserted().GetMessage().GetKind() == hearthv1.MessageKind_MESSAGE_KIND_APPROVAL_REQUEST
	})
	if waiting < 0 || asked < 0 {
		t.Fatalf("run_changed(waiting_owner) at %d, message_upserted(approval_request) at %d; events %v", waiting, asked, evs)
	}
	opened := evs[len(evs)-1].GetInboxChanged().GetItem()
	if opened.GetKind() != hearthv1.InboxKind_INBOX_KIND_APPROVAL || opened.GetCall().GetName() != "files.delete" ||
		opened.GetEffect() != "Delete brief.md from Dana's folder" || opened.GetConversationId() != bot.GetDmConversationId() {
		t.Fatalf("inbox_changed = %v", opened)
	}

	open := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{}).GetItems()
	if len(open) != 1 || open[0].GetId() != opened.GetId() {
		t.Fatalf("ListInbox = %v, want the one open item", open)
	}
	conv := unary(t, c.conv.GetConversation, &hearthv1.GetConversationRequest{Id: bot.GetDmConversationId()}).GetConversation()
	if conv.GetLatestRun().GetId() != sent.GetRun().GetId() || conv.GetLatestRun().GetState() != hearthv1.RunState_RUN_STATE_WAITING_OWNER {
		t.Fatalf("GetConversation latest_run = %v, want the waiting run", conv.GetLatestRun())
	}

	decide := func(d hearthv1.Decision) (*hearthv1.DecideResponse, error) {
		res, err := c.inbox.Decide(ctx, connect.NewRequest(&hearthv1.DecideRequest{ItemId: opened.GetId(), Decision: d}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	if _, err := decide(hearthv1.Decision_DECISION_UNSPECIFIED); code(err) != connect.CodeInvalidArgument {
		t.Errorf("Decide with no decision = %v, want invalid_argument", err)
	}
	if _, err := decide(hearthv1.Decision_DECISION_ALWAYS_ALLOW); code(err) != connect.CodeInvalidArgument {
		t.Errorf("Decide always allow on a delete = %v, want invalid_argument", err)
	}
	if _, err := os.Stat(brief); err != nil {
		t.Fatalf("a refused decision deleted brief.md: %v", err)
	}
	if _, err := c.inbox.Decide(ctx, connect.NewRequest(&hearthv1.DecideRequest{ItemId: "nope", Decision: hearthv1.Decision_DECISION_DENY})); code(err) != connect.CodeNotFound {
		t.Errorf("Decide on an unknown item = %v, want not_found", err)
	}

	decided, err := decide(hearthv1.Decision_DECISION_ALLOW_ONCE)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if got := decided.GetItem(); got.GetState() != hearthv1.InboxState_INBOX_STATE_CLOSED ||
		got.GetDecision() != hearthv1.Decision_DECISION_ALLOW_ONCE || got.GetOutcome() == "" || got.GetClosedAt() == nil {
		t.Errorf("Decide returned the item %v, want it closed", got)
	}
	resumed := decided.GetRun()
	if resumed.GetId() == "" || resumed.GetId() == sent.GetRun().GetId() {
		t.Errorf("Decide returned the run %v, want a new run", resumed)
	}
	f.until(func(m *hearthv1.SubscribeResponse) bool {
		return runEnded(m) && m.GetRunChanged().GetRun().GetId() == resumed.GetId()
	})
	if _, err := os.Stat(brief); !os.IsNotExist(err) {
		t.Errorf("brief.md after Allow once: %v, want it deleted", err)
	}

	if _, err := decide(hearthv1.Decision_DECISION_ALLOW_ONCE); code(err) != connect.CodeFailedPrecondition {
		t.Errorf("Decide on a closed item = %v, want failed_precondition", err)
	}
	if left := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{}).GetItems(); len(left) != 0 {
		t.Errorf("ListInbox after the decision = %v, want nothing open", left)
	}
	if all := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{IncludeClosed: true}).GetItems(); len(all) != 1 {
		t.Errorf("ListInbox with closed items = %v, want the one", all)
	}
}

func TestApproval_ParksThenAllowsOnce(t *testing.T) {
	r := newRig(t, calls("c1", "files.delete", `{"path":"brief.md"}`), answer("Deleted."))
	checkApprovalFlow(t, newClients(serve(t, r.opts), testToken), r)
}

func TestGetConversation_MidRunCarriesTheWorkingRun(t *testing.T) {
	r := newRig(t, hang("working"))
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)
	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "go"})
	f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetMessageDelta() != nil })

	got := unary(t, c.conv.GetConversation, &hearthv1.GetConversationRequest{Id: bot.GetDmConversationId()}).GetConversation()
	if got.GetLatestRun().GetId() != sent.GetRun().GetId() || got.GetLatestRun().GetState() != hearthv1.RunState_RUN_STATE_WORKING {
		t.Errorf("latest_run = %v, want the working run", got.GetLatestRun())
	}
	convs := unary(t, c.conv.ListConversations, &hearthv1.ListConversationsRequest{BotId: bot.GetId()}).GetConversations()
	if len(convs) != 1 || convs[0].GetLatestRun().GetId() != sent.GetRun().GetId() {
		t.Errorf("ListConversations = %v, want latest_run on the row", convs)
	}
	if _, err := c.conv.Retry(t.Context(), connect.NewRequest(&hearthv1.RetryRequest{RunId: sent.GetRun().GetId()})); code(err) != connect.CodeFailedPrecondition {
		t.Errorf("Retry of a working run = %v, want failed_precondition", err)
	}
	if _, err := c.conv.GetConversation(t.Context(), connect.NewRequest(&hearthv1.GetConversationRequest{Id: "nope"})); code(err) != connect.CodeNotFound {
		t.Errorf("GetConversation of an unknown id = %v, want not_found", err)
	}
	if _, err := c.conv.Retry(t.Context(), connect.NewRequest(&hearthv1.RetryRequest{RunId: "nope"})); code(err) != connect.CodeNotFound {
		t.Errorf("Retry of an unknown run = %v, want not_found", err)
	}
}

func TestRetry_AFailedRunGetsANewOneAndItsItemLinksToTheThread(t *testing.T) {
	r := newRig(t, broken(), answer("Here you go."))
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)
	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "hello"})
	evs := f.until(inboxOpened)
	if end := evs[indexOf(evs, runEnded)].GetRunChanged().GetRun(); end.GetState() != hearthv1.RunState_RUN_STATE_FAILED || end.GetId() != sent.GetRun().GetId() {
		t.Fatalf("the run ended %v, want failed", end)
	}

	items := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{}).GetItems()
	if len(items) != 1 || items[0].GetKind() != hearthv1.InboxKind_INBOX_KIND_FAILURE ||
		items[0].GetRunId() != sent.GetRun().GetId() || items[0].GetConversationId() != bot.GetDmConversationId() {
		t.Fatalf("ListInbox = %v, want the failure linked to the thread through its run", items)
	}
	if got := evs[len(evs)-1].GetInboxChanged().GetItem().GetConversationId(); got != bot.GetDmConversationId() {
		t.Errorf("inbox_changed conversation = %q, want the thread", got)
	}

	retried := unary(t, c.conv.Retry, &hearthv1.RetryRequest{RunId: sent.GetRun().GetId()}).GetRun()
	if retried.GetId() == "" || retried.GetId() == sent.GetRun().GetId() || retried.GetConversationId() != bot.GetDmConversationId() {
		t.Fatalf("Retry = %v, want a new run in the thread", retried)
	}
	f.until(func(m *hearthv1.SubscribeResponse) bool {
		return runEnded(m) && m.GetRunChanged().GetRun().GetId() == retried.GetId()
	})
	if open := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{}).GetItems(); len(open) != 0 {
		t.Errorf("ListInbox after the retry = %v, want the failure closed", open)
	}
}

func TestAcknowledge_ClosesAFailureAndRefusesAnApproval(t *testing.T) {
	r := newRig(t, broken())
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)
	unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "hello"})
	item := f.until(inboxOpened)
	id := item[len(item)-1].GetInboxChanged().GetItem().GetId()

	got := unary(t, c.inbox.Acknowledge, &hearthv1.AcknowledgeRequest{ItemId: id}).GetItem()
	if got.GetState() != hearthv1.InboxState_INBOX_STATE_CLOSED || got.GetOutcome() != "Acknowledged" {
		t.Errorf("Acknowledge = %v", got)
	}
	if _, err := c.inbox.Acknowledge(t.Context(), connect.NewRequest(&hearthv1.AcknowledgeRequest{ItemId: id})); code(err) != connect.CodeFailedPrecondition {
		t.Errorf("Acknowledge twice = %v, want failed_precondition", err)
	}
	if _, err := c.inbox.Answer(t.Context(), connect.NewRequest(&hearthv1.AnswerRequest{ItemId: id, Answer: "hi"})); code(err) != connect.CodeInvalidArgument {
		t.Errorf("Answer to a failure = %v, want invalid_argument", err)
	}
	if _, err := c.inbox.Acknowledge(t.Context(), connect.NewRequest(&hearthv1.AcknowledgeRequest{})); code(err) != connect.CodeInvalidArgument {
		t.Errorf("Acknowledge with no item = %v, want invalid_argument", err)
	}
}

func TestGetOrg_BeforeOnboardingIsEmptyNotMissing(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	got, err := c.org.GetOrg(t.Context(), connect.NewRequest(&hearthv1.GetOrgRequest{}))
	if err != nil {
		t.Fatalf("GetOrg on a fresh store: %v", err)
	}
	if got.Msg.GetOrg() != nil {
		t.Errorf("GetOrg = %v, want no org", got.Msg)
	}
}

// ask is a model reply asking the owner a question with a control.
func ask(question, widget string) step {
	return calls("q1", "ask_owner", `{"question":"`+question+`","widget":`+widget+`}`)
}

func TestAnswer_ToTheControlsOfAQuestion(t *testing.T) {
	day24 := timestamppb.New(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC))
	tests := []struct {
		name   string
		widget string
		value  *hearthv1.WidgetAnswer
		want   string // the fixed words the bot reads
		check  func(t *testing.T, got *hearthv1.WidgetAnswer)
	}{
		{
			name:   "a single choice",
			widget: `{"kind":"single_choice","options":[{"id":"keep","label":"Keep it"},{"id":"drop","label":"Drop it"}]}`,
			value:  &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_ChoiceId{ChoiceId: "keep"}},
			want:   "The owner chose: Keep it",
			check: func(t *testing.T, got *hearthv1.WidgetAnswer) {
				if got.GetChoiceId() != "keep" {
					t.Errorf("choice_id = %q", got.GetChoiceId())
				}
			},
		},
		{
			name:   "other text with no value",
			widget: `{"kind":"single_choice","allow_other":true,"options":[{"id":"keep","label":"Keep it"},{"id":"drop","label":"Drop it"}]}`,
			value:  &hearthv1.WidgetAnswer{Other: "Neither"},
			want:   "The owner chose: Other: Neither",
			check: func(t *testing.T, got *hearthv1.WidgetAnswer) {
				if got.GetOther() != "Neither" || got.GetValue() != nil {
					t.Errorf("answer = %v, want other alone", got)
				}
			},
		},
		{
			name:   "a day sent as UTC midnight",
			widget: `{"kind":"date"}`,
			value:  &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Date{Date: day24}},
			want:   "The owner picked the date: 2026-10-24",
			check: func(t *testing.T, got *hearthv1.WidgetAnswer) {
				if !got.GetDate().AsTime().Equal(day24.AsTime()) {
					t.Errorf("date = %v, want %v", got.GetDate().AsTime(), day24.AsTime())
				}
			},
		},
		{
			name:   "a date with a time",
			widget: `{"kind":"date","with_time":true}`,
			value:  &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Date{Date: timestamppb.New(day24.AsTime().Add(14*time.Hour + 30*time.Minute))}},
			want:   "The owner picked the date: 2026-10-24 14:30",
			check: func(t *testing.T, got *hearthv1.WidgetAnswer) {
				if want := day24.AsTime().Add(14*time.Hour + 30*time.Minute); !got.GetDate().AsTime().Equal(want) {
					t.Errorf("date = %v, want %v", got.GetDate().AsTime(), want)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, ask("Which one?", tt.widget), answer("Thanks."))
			c := newClients(serve(t, r.opts), testToken)
			bot := hireDana(t, c, r)
			f := follow(t, c, r.bus)
			unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "ask me"})
			asked := f.until(inboxOpened)
			item := asked[len(asked)-1].GetInboxChanged().GetItem()
			if item.GetKind() != hearthv1.InboxKind_INBOX_KIND_QUESTION || item.GetWidget() == nil {
				t.Fatalf("inbox_changed = %v, want a question with a control", item)
			}

			res := unary(t, c.inbox.Answer, &hearthv1.AnswerRequest{ItemId: item.GetId(), Value: tt.value})
			if got := res.GetItem().GetAnswerValue(); got.GetTextForm() != tt.want {
				t.Errorf("Answer text_form = %q, want %q", got.GetTextForm(), tt.want)
			} else {
				tt.check(t, got)
			}

			evs := f.until(func(m *hearthv1.SubscribeResponse) bool {
				return m.GetInboxChanged().GetItem().GetState() == hearthv1.InboxState_INBOX_STATE_CLOSED
			})
			shown := indexOf(evs, func(m *hearthv1.SubscribeResponse) bool {
				msg := m.GetMessageUpserted().GetMessage()
				return msg.GetKind() == hearthv1.MessageKind_MESSAGE_KIND_QUESTION && msg.GetAnswer() != nil
			})
			if shown < 0 || shown >= len(evs)-1 {
				t.Errorf("the question with its answer came at %d of %d events; it must precede the item closing", shown, len(evs))
			}
			f.until(func(m *hearthv1.SubscribeResponse) bool {
				return runEnded(m) && m.GetRunChanged().GetRun().GetId() == res.GetRun().GetId()
			})
		})
	}
}

func TestAnswer_RefusesWhatTheControlDoesNot(t *testing.T) {
	r := newRig(t, ask("Which one?", `{"kind":"single_choice","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}`))
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)
	unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: bot.GetDmConversationId(), Text: "ask me"})
	asked := f.until(inboxOpened)
	id := asked[len(asked)-1].GetInboxChanged().GetItem().GetId()

	tests := []struct {
		name string
		req  *hearthv1.AnswerRequest
		want connect.Code
	}{
		{"no item", &hearthv1.AnswerRequest{Answer: "a"}, connect.CodeInvalidArgument},
		{"an option that is not offered", &hearthv1.AnswerRequest{ItemId: id, Value: &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_ChoiceId{ChoiceId: "z"}}}, connect.CodeInvalidArgument},
		{"the wrong kind of answer", &hearthv1.AnswerRequest{ItemId: id, Value: &hearthv1.WidgetAnswer{Value: &hearthv1.WidgetAnswer_Yes{Yes: true}}}, connect.CodeInvalidArgument},
		{"nothing at all", &hearthv1.AnswerRequest{ItemId: id}, connect.CodeInvalidArgument},
		{"an unknown item", &hearthv1.AnswerRequest{ItemId: "nope", Answer: "a"}, connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := c.inbox.Answer(t.Context(), connect.NewRequest(tt.req)); code(err) != tt.want {
				t.Errorf("Answer = %v, want %v", err, tt.want)
			}
		})
	}
	if open := unary(t, c.inbox.ListInbox, &hearthv1.ListInboxRequest{}).GetItems(); len(open) != 1 {
		t.Errorf("the refused answers left %d open items, want 1", len(open))
	}
}

func TestEvents_MessageDeletedConversationChangedAndGrantsChanged(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	f := follow(t, c, r.bus)

	r.bus.Publish(orchestrate.MessageDeleted{ConversationID: "c1", MessageID: "m1"})
	r.bus.Publish(ConversationChanged{Conversation: org.Conversation{ID: "c1", BotID: bot.GetId(), LatestRun: &org.Run{ID: "r1", State: org.RunDone}}})
	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetConversationChanged() != nil })
	if d := evs[0].GetMessageDeleted(); d.GetConversationId() != "c1" || d.GetMessageId() != "m1" {
		t.Errorf("message_deleted = %v", evs[0])
	}
	if got := evs[1].GetConversationChanged().GetConversation(); got.GetId() != "c1" || got.GetLatestRun().GetState() != hearthv1.RunState_RUN_STATE_DONE {
		t.Errorf("conversation_changed = %v", evs[1])
	}
}
