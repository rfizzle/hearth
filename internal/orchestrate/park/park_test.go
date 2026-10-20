package park

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// thread numbers msgs as the store would.
func thread(msgs ...org.Message) []org.Message {
	for i := range msgs {
		msgs[i].Seq = int64(i + 1)
	}
	return msgs
}

func text(run, s string) org.Message {
	return org.Message{RunID: run, Kind: org.MessageText, Content: s}
}
func call(run, id string) org.Message {
	return org.Message{RunID: run, Kind: org.MessageToolCall, ToolCalls: []org.ToolCall{{ID: id, Name: "t." + id, Arguments: "{}"}}}
}
func result(run, id, s string) org.Message {
	return org.Message{RunID: run, Kind: org.MessageToolResult, ToolCallID: id, Content: s}
}
func requested(run string) org.Message {
	return org.Message{RunID: run, Kind: org.MessageApprovalRequest}
}

func TestFind(t *testing.T) {
	owner := org.Message{Kind: org.MessageText, Content: "go"}
	cases := []struct {
		name    string
		msgs    []org.Message
		start   int64
		text    string
		calls   []string
		results map[string]string
	}{
		{"one gated call after text", thread(owner, text("r", "On it."), call("r", "a"), requested("r"), result("r", "a", "done")),
			2, "On it.", []string{"a"}, map[string]string{"a": "done"}},
		{"an auto call before the gated one", thread(owner, call("r", "a"), call("r", "b"), result("r", "a", "ran"), call("r", "c"), requested("r")),
			2, "", []string{"a", "b", "c"}, map[string]string{"a": "ran"}},
		{"an earlier round is left out", thread(owner, call("r", "a"), result("r", "a", "ran"), call("r", "b"), result("r", "b", "ran"), call("r", "c"), requested("r")),
			4, "", []string{"b", "c"}, map[string]string{"a": "ran", "b": "ran"}},
		{"an earlier run is left out", thread(text("old", "Hi."), call("old", "x"), owner, call("r", "a"), requested("r")),
			4, "", []string{"a"}, map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := Find(tc.msgs, "r")
			if !ok {
				t.Fatal("no round found")
			}
			var ids []string
			for _, c := range r.Calls {
				ids = append(ids, c.ID)
			}
			if r.Start != tc.start || r.Text != tc.text || !slices.Equal(ids, tc.calls) {
				t.Fatalf("round = start %d text %q calls %v, want %d %q %v", r.Start, r.Text, ids, tc.start, tc.text, tc.calls)
			}
			for id, want := range tc.results {
				if r.Results[id] != want {
					t.Errorf("result %s = %q, want %q", id, r.Results[id], want)
				}
			}
		})
	}
	if _, ok := Find(thread(owner, call("r", "a")), "r"); ok {
		t.Error("a run with no request has a parked round")
	}
}

func TestResume_Assemble(t *testing.T) {
	r := Round{Text: "Tidying up.", Calls: []org.ToolCall{{ID: "a", Name: "files.write", Arguments: `{"path":"n.md"}`}, {ID: "b", Name: "files.delete", Arguments: `{"path":"x.md"}`}},
		Results: map[string]string{"b": "Deleted x.md."}}
	history := []agent.Message{{Role: agent.RoleOwner, Text: "tidy"}, {Role: agent.RoleBot, Text: "Sure."}}

	t.Run("structured", func(t *testing.T) {
		msgs, prompt := Resume{}.Assemble(slices.Clone(history), r, Note)
		if prompt != Note || len(msgs) != 4 {
			t.Fatalf("prompt %q, messages %+v", prompt, msgs)
		}
		bot := msgs[1]
		if bot.Role != agent.RoleBot || bot.Text != "Sure.\n\nTidying up." || len(bot.Calls) != 2 || string(bot.Calls[1].Args) != `{"path":"x.md"}` {
			t.Errorf("bot = %+v, want the text before joined to the round's, with both calls", bot)
		}
		if m := msgs[2]; m.Role != agent.RoleTool || m.CallID != "a" || m.Text != NotRun {
			t.Errorf("first result = %+v, want not run", msgs[2])
		}
		if msgs[3].CallID != "b" || msgs[3].Text != "Deleted x.md." {
			t.Errorf("second result = %+v", msgs[3])
		}
	})

	t.Run("as text", func(t *testing.T) {
		msgs, prompt := Resume{AsText: true}.Assemble(slices.Clone(history), r, Note)
		if len(msgs) != 2 {
			t.Fatalf("messages = %+v, want the history alone", msgs)
		}
		for _, want := range []string{"(hearth) Before you stopped", "Tidying up.", `files.delete with {"path":"x.md"}. The result: Deleted x.md.`, "files.write", NotRun} {
			if !strings.Contains(prompt, want) {
				t.Errorf("prompt is missing %q:\n%s", want, prompt)
			}
		}
		if !strings.HasSuffix(prompt, "\n\n"+Note) {
			t.Errorf("prompt does not end with the note:\n%s", prompt)
		}
	})
}

func TestWords(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{Allowed(inbox.AllowOnce, "Dana", "Other"), "Allowed once"},
		{Allowed(inbox.AllowForTask, "Dana", "Other"), "Allowed for this conversation in Other"},
		{Allowed(inbox.AlwaysAllow, "Dana", "Other"), "Always allowed for Dana in Other"},
		{Allowed(inbox.AlwaysAllow, "Dana", ""), "Always allowed for Dana"},
		{Declined(""), "Declined"},
		{Declined("not that one"), "Declined: not that one"},
		{Denial(""), "The owner declined this. Do not try it again in this conversation; do what you can without it."},
		{Denial("not that one"), "The owner declined this. Reason: not that one. Do not try it again in this conversation; do what you can without it."},
		{Answered("Tuesday"), "The owner answered: Tuesday"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestRequest(t *testing.T) {
	run := org.Run{ID: "r", BotID: "b", ConversationID: "c"}
	cases := []struct {
		name     string
		g        Call
		kind     org.MessageKind
		itemKind inbox.Kind
		content  string
		widget   string
	}{
		{"an approval is in the describer's words", Call{ID: "1", Name: "files.delete", Args: json.RawMessage(`{"path":"a"}`), Effect: "Delete a"},
			org.MessageApprovalRequest, inbox.Approval, "Delete a", ""},
		{"a question is its own words, with the widget untouched", Call{ID: "2", Name: "ask_owner",
			Args: json.RawMessage(`{"question":"Keep it?","suggested_answers":["Yes"],"widget":{"kind":"yes_no"}}`), Effect: "Ask you: Keep it?"},
			org.MessageQuestion, inbox.Question, "Keep it?", `{"kind":"yes_no"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, it := Request(run, tc.g)
			if m.Kind != tc.kind || m.Content != tc.content || m.Widget != tc.widget || len(m.ToolCalls) != 1 || m.ToolCalls[0].ID != tc.g.ID {
				t.Fatalf("message = %+v", m)
			}
			if it.Kind != tc.itemKind || it.RunID != "r" || it.BotID != "b" || it.ConversationID != "c" || it.Effect != tc.g.Effect ||
				it.Call == nil || it.Call.Arguments != string(tc.g.Args) || it.Widget != tc.widget {
				t.Fatalf("item = %+v", it)
			}
			if tc.itemKind == inbox.Question && (it.Question != "Keep it?" || !slices.Equal(it.Suggested, []string{"Yes"})) {
				t.Fatalf("question item = %+v", it)
			}
		})
	}
}
