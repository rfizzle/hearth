package park

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

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

func TestRaise(t *testing.T) {
	grants := []org.Grant{
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: "/A/B"},
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: "/A"},
		{Capability: org.FilesRead, Mode: org.ModeAsk, Folder: "/A/B"},
	}
	got := Raise(grants, org.FilesWrite, "/A/B")
	want := []org.Mode{org.ModeAllowed, org.ModeAsk, org.ModeAsk}
	for i, g := range got {
		if g.Mode != want[i] {
			t.Errorf("grant %d = %+v, want mode %d", i, g, want[i])
		}
	}
	if grants[0].Mode != org.ModeAsk {
		t.Error("Raise changed the grants it was given")
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
