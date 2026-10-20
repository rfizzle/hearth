package orchestrate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/widget"
)

const threeWays = `{"kind":"single_choice","options":[{"id":"a","label":"Keep it"},{"id":"b","label":"Archive it"},{"id":"c","label":"Delete it"}]}`

// askFixture is a fixture whose bot asks args, then says it is done.
func askFixture(t *testing.T, args string) *fixture {
	t.Helper()
	f := newFixture(t,
		events(llm.StreamEvent{Token: "One question first."}, toolCall("q1", "ask_owner", args)),
		events(llm.StreamEvent{Token: "Thanks."}, llm.StreamEvent{Done: true}),
	)
	f.svc.grace = 10 * time.Millisecond
	return f
}

// answer answers the item and waits for the run that carries on to end,
// returning the item as Answer gave it and that run.
func (f *fixture) answer(t *testing.T, it inbox.Item, text string, value *widget.Answer) (inbox.Item, org.Run) {
	t.Helper()
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	got, waiting, err := f.svc.Answer(context.Background(), it.ID, text, value)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if waiting.ID != it.RunID || waiting.State != org.RunWaitingOwner {
		t.Fatalf("Answer returned run %+v, want the one still waiting", waiting)
	}
	evs := until(t, ch, func(e Event) bool { return ended(e) && e.Body.(RunChanged).Run.ID != it.RunID })
	return got, f.storedRun(t, evs[len(evs)-1].Body.(RunChanged).Run.ID)
}

// resumedResult is the tool message the resumed request carried for the
// question.
func (f *fixture) resumedResult(t *testing.T) string {
	t.Helper()
	tail := sentTail(t, f.script.seen[1])
	if len(tail) != 3 || tail[1].Role != llm.RoleTool || tail[1].ToolCallID != "q1" {
		t.Fatalf("resumed request after the owner's message = %+v, want the round and the note", tail)
	}
	return tail[1].Content
}

func TestQuestion_InWordsParksAndTheAnswerResumes(t *testing.T) {
	f := askFixture(t, `{"question":"Which month?","suggested_answers":["March","April"]}`)
	parked, it := f.park(t, "do the report")
	if it.Kind != inbox.Question || it.Question != "Which month?" || len(it.Suggested) != 2 || it.Widget != "" || it.RunID != parked.ID {
		t.Fatalf("item = %+v, want the question", it)
	}
	if it.Effect != "Ask you: Which month?" {
		t.Errorf("effect = %q, want the describer's words", it.Effect)
	}
	msgs := f.messages(t)
	if q := msgs[len(msgs)-1]; q.Kind != org.MessageQuestion || q.Content != "Which month?" || q.ID != it.MessageID {
		t.Fatalf("last message = %+v, want the question", q)
	}
	if _, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "hello?", nil); !errors.Is(err, ErrWaiting) {
		t.Fatalf("Send while asked = %v, want ErrWaiting", err)
	}

	_, resumed := f.answer(t, it, "April", nil)
	if resumed.State != org.RunDone {
		t.Fatalf("resumed run = %+v, want done", resumed)
	}
	if got := f.resumedResult(t); got != "The owner answered: April" {
		t.Fatalf("the bot read %q", got)
	}
	closed, err := f.db.InboxItem(context.Background(), it.ID)
	if err != nil || closed.State != inbox.Closed || closed.Answer != "April" {
		t.Fatalf("item = %+v, %v, want closed with the answer", closed, err)
	}
	var owner *org.Message
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageText && m.AuthorBotID == "" && m.Content == "April" {
			owner = &m
		}
	}
	if owner == nil {
		t.Fatal("the owner's answer is not in the thread")
	}
}

func TestQuestion_ASingleChoice(t *testing.T) {
	f := askFixture(t, `{"question":"What should I do with the old file?","widget":`+threeWays+`}`)
	_, it := f.park(t, "tidy up")
	msgs := f.messages(t)
	q := msgs[len(msgs)-1]
	if q.Kind != org.MessageQuestion || q.Widget != threeWays || it.Widget != threeWays {
		t.Fatalf("question %+v, item widget %q, want the spec on both", q, it.Widget)
	}
	ctx := context.Background()
	if _, _, err := f.svc.Answer(ctx, it.ID, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"z"}}); !errors.Is(err, widget.ErrInvalid) {
		t.Fatalf("an unknown choice = %v, want widget.ErrInvalid", err)
	}
	if got, _ := f.db.InboxItem(ctx, it.ID); got.State != inbox.Open {
		t.Fatal("a refused answer closed the item")
	}

	got, resumed := f.answer(t, it, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"b"}})
	if got.State != inbox.Open || got.Answer != "Archive it" {
		t.Errorf("Answer gave the item %+v, want it open with the answer during the grace", got)
	}
	if resumed.State != org.RunDone {
		t.Fatalf("resumed run = %+v", resumed)
	}
	if res := f.resumedResult(t); res != "The owner chose: Archive it" {
		t.Fatalf("the bot read %q", res)
	}
	stored, err := f.db.Message(ctx, q.ID)
	if err != nil || stored.WidgetAnswer != `{"kind":"single_choice","choices":["b"]}` {
		t.Fatalf("question = %+v, %v, want it to carry the answer", stored, err)
	}
	if closed, _ := f.db.InboxItem(ctx, it.ID); closed.State != inbox.Closed || closed.WidgetAnswer != stored.WidgetAnswer || closed.Answer != "Archive it" {
		t.Errorf("item = %+v, want it closed with the answer", closed)
	}
	var said string
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageText && m.AuthorBotID == "" && m.Seq > q.Seq {
			said = m.Content
		}
	}
	if said != "Archive it" {
		t.Errorf("the owner's message reads %q, want their choice", said)
	}
}

func TestQuestion_FreeTextInTheControl(t *testing.T) {
	f := askFixture(t, `{"question":"What should the title be?","widget":{"kind":"free_text","max_length":40}}`)
	_, it := f.park(t, "name it")
	f.answer(t, it, "", &widget.Answer{Kind: widget.FreeText, Text: "Q3 in brief"})
	if got := f.resumedResult(t); got != "The owner answered: Q3 in brief" {
		t.Fatalf("the bot read %q", got)
	}
}

func TestQuestion_AMalformedControlIsRefusedWithoutParking(t *testing.T) {
	f := newFixture(t,
		events(toolCall("q1", "ask_owner", `{"question":"Which?","widget":{"kind":"single_choice","options":[{"id":"a","label":"Only"}]}}`)),
		events(llm.StreamEvent{Token: "I will ask in words."}, llm.StreamEvent{Done: true}),
	)
	r, _ := f.send(t, "go", ended)
	if got := f.storedRun(t, r.ID); got.State != org.RunDone {
		t.Fatalf("run = %+v, want done without waiting", got)
	}
	if items, err := f.db.InboxItems(context.Background(), false); err != nil || len(items) != 0 {
		t.Fatalf("items = %+v, %v, want none", items, err)
	}
	var result string
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageToolResult && m.ToolCallID == "q1" {
			result = m.Content
		}
	}
	if want := "ask_owner refused: a single choice needs 2 to 8 options; you gave 1"; result != want {
		t.Fatalf("the bot read %q, want %q", result, want)
	}
}

func TestQuestion_ASecondAnswerInsideTheGraceReplacesTheFirst(t *testing.T) {
	f := askFixture(t, `{"question":"What should I do with it?","widget":`+threeWays+`}`)
	f.svc.grace = 300 * time.Millisecond
	_, it := f.park(t, "tidy up")
	if _, _, err := f.svc.Answer(context.Background(), it.ID, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"c"}}); err != nil {
		t.Fatal(err)
	}
	f.answer(t, it, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"a"}})
	if got := f.resumedResult(t); got != "The owner chose: Keep it" {
		t.Fatalf("the bot read %q, want the second answer", got)
	}
	if n := len(f.script.seen); n != 2 {
		t.Fatalf("%d requests, want one resumed run", n)
	}
	var owners int
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageText && m.AuthorBotID == "" {
			owners++
		}
	}
	if owners != 2 {
		t.Fatalf("%d owner messages, want the request and one answer", owners)
	}
}

func TestQuestion_WordsInsideTheGraceTakeTheChoiceOffTheCard(t *testing.T) {
	f := askFixture(t, `{"question":"What should I do with it?","widget":`+threeWays+`}`)
	f.svc.grace = 300 * time.Millisecond
	_, it := f.park(t, "tidy up")
	if _, _, err := f.svc.Answer(context.Background(), it.ID, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"c"}}); err != nil {
		t.Fatal(err)
	}
	f.answer(t, it, "actually, neither", nil)
	if got := f.resumedResult(t); got != "The owner answered: actually, neither" {
		t.Fatalf("the bot read %q", got)
	}
	q, err := f.db.Message(context.Background(), it.MessageID)
	if err != nil || q.WidgetAnswer != "" {
		t.Fatalf("question = %+v, %v, want the earlier choice taken off", q, err)
	}
}
