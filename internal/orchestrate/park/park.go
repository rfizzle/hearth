package park

import (
	"encoding/json"
	"slices"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// Call is a call a run parked on, as the owner is asked about it: the call,
// the tool's own words for what it would do, and for a question the control
// it asks with, as the JSON the model sent.
type Call struct {
	ID     string
	Name   string
	Args   json.RawMessage
	Effect string
	Widget string
}

// question is the arguments of a call that asks the owner something.
type question struct {
	Question  string          `json:"question"`
	Suggested []string        `json:"suggested_answers"`
	Widget    json.RawMessage `json:"widget"`
}

// Request is the thread message and the Inbox item a run r parked on c
// leaves: an approval request in the tool's own words, or, for a call that
// asks the owner, the question with its control untouched. The message's id
// is not known until it is stored, so the item's MessageID is left for the
// caller.
// See docs/model.md#8-approvals.
func Request(r org.Run, c Call) (org.Message, inbox.Item) {
	call := org.ToolCall{ID: c.ID, Name: c.Name, Arguments: string(c.Args), Summary: c.Effect}
	m := org.Message{Kind: org.MessageApprovalRequest, Content: c.Effect, ToolCalls: []org.ToolCall{call}}
	it := inbox.Item{
		Kind: inbox.Approval, BotID: r.BotID, ConversationID: r.ConversationID, RunID: r.ID,
		Title: c.Effect, Effect: c.Effect, Call: &call,
	}
	if c.Name != string(org.AskOwner) {
		return m, it
	}
	var q question
	// Arguments that do not parse ask nothing in words; the call is still
	// the owner's to answer.
	_ = json.Unmarshal(c.Args, &q)
	if c.Widget == "" && len(q.Widget) > 0 && string(q.Widget) != "null" {
		c.Widget = string(q.Widget)
	}
	m.Kind, m.Content, m.Widget = org.MessageQuestion, q.Question, c.Widget
	it.Kind, it.Title, it.Question, it.Suggested, it.Widget = inbox.Question, q.Question, q.Question, q.Suggested, c.Widget
	return m, it
}

// Allowed is the thread's words for an allow decision on a call of
// botName's in folder, the name of the granted folder the call fell under,
// or empty for a capability with no folder. A decision that outlasts the
// call names the folder it widens.
func Allowed(d inbox.Decision, botName, folder string) string {
	in := ""
	if folder != "" {
		in = " in " + folder
	}
	switch d {
	case inbox.AllowForTask:
		return "Allowed for this conversation" + in
	case inbox.AlwaysAllow:
		return "Always allowed for " + botName + in
	}
	return "Allowed once"
}

// Raise is grants with the ask-mode grant of capability in granted, the
// folder as the grant names it, raised to allowed, and no other: always
// allowing a write in one folder must not allow a folder around it that
// happens to be granted too. The caller reads the grants just before, so
// an edit in the Access sheet is not undone.
// See docs/model.md#8-approvals.
func Raise(grants []org.Grant, capability org.Capability, granted string) []org.Grant {
	out := slices.Clone(grants)
	for i, g := range out {
		if g.Capability == capability && g.Folder == granted && g.Mode == org.ModeAsk {
			out[i].Mode = org.ModeAllowed
		}
	}
	return out
}

// Declined is the thread's words for a denial.
func Declined(reason string) string {
	if reason == "" {
		return "Declined"
	}
	return "Declined: " + reason
}

// Denial is the result a denied call gives the bot: the owner's reason, if
// they gave one, and not to try again.
func Denial(reason string) string {
	const rest = "Do not try it again in this conversation; do what you can without it."
	if reason == "" {
		return "The owner declined this. " + rest
	}
	return "The owner declined this. Reason: " + reason + ". " + rest
}

// Answered is the result a question's call gives the bot: the owner's words.
func Answered(text string) string {
	return "The owner answered: " + text
}
