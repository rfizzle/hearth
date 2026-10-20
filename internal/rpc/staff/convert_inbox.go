package staff

import (
	"context"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// The conversions for the Inbox and the grants. Every enum goes through a
// table, never a cast, because the domain's numbers are stored on disk and
// the contract's are appended to; a test holds each table to both lists.

// capabilities is the contract's capability enum against the domain's. The
// right to ask the owner has no entry: every bot has it and the owner does
// not edit it.
var capabilities = map[hearthv1.Capability]org.Capability{
	hearthv1.Capability_CAPABILITY_FILES_READ:   org.FilesRead,
	hearthv1.Capability_CAPABILITY_FILES_WRITE:  org.FilesWrite,
	hearthv1.Capability_CAPABILITY_FILES_DELETE: org.FilesDelete,
	hearthv1.Capability_CAPABILITY_WEB_FETCH:    org.WebFetch,
	hearthv1.Capability_CAPABILITY_WEB_SEARCH:   org.WebSearch,
}

// capabilityRows is the order the Access sheet lists the capabilities in,
// and the plain words for each.
var capabilityRows = []struct {
	capability hearthv1.Capability
	title      string
}{
	{hearthv1.Capability_CAPABILITY_FILES_READ, "Read files"},
	{hearthv1.Capability_CAPABILITY_FILES_WRITE, "Write files"},
	{hearthv1.Capability_CAPABILITY_FILES_DELETE, "Delete files"},
	{hearthv1.Capability_CAPABILITY_WEB_FETCH, "Read web pages"},
	{hearthv1.Capability_CAPABILITY_WEB_SEARCH, "Search the web"},
}

var grantModes = map[hearthv1.GrantMode]org.Mode{
	hearthv1.GrantMode_GRANT_MODE_NOT_GRANTED: org.ModeNotGranted,
	hearthv1.GrantMode_GRANT_MODE_ASK:         org.ModeAsk,
	hearthv1.GrantMode_GRANT_MODE_ALLOWED:     org.ModeAllowed,
}

var decisions = map[hearthv1.Decision]inbox.Decision{
	hearthv1.Decision_DECISION_ALLOW_ONCE:     inbox.AllowOnce,
	hearthv1.Decision_DECISION_ALLOW_FOR_TASK: inbox.AllowForTask,
	hearthv1.Decision_DECISION_ALWAYS_ALLOW:   inbox.AlwaysAllow,
	hearthv1.Decision_DECISION_DENY:           inbox.Deny,
}

var inboxKinds = map[hearthv1.InboxKind]inbox.Kind{
	hearthv1.InboxKind_INBOX_KIND_APPROVAL: inbox.Approval,
	hearthv1.InboxKind_INBOX_KIND_QUESTION: inbox.Question,
	hearthv1.InboxKind_INBOX_KIND_FAILURE:  inbox.Failure,
	hearthv1.InboxKind_INBOX_KIND_BUDGET:   inbox.Budget,
}

var inboxStates = map[hearthv1.InboxState]inbox.State{
	hearthv1.InboxState_INBOX_STATE_OPEN:   inbox.Open,
	hearthv1.InboxState_INBOX_STATE_CLOSED: inbox.Closed,
}

// grantFromProto is the grant the owner set on a row of the Access sheet.
// The folder is not judged here: the store keeps the bot's own, and the
// tools check every path against what is granted.
func grantFromProto(g *hearthv1.Grant) (org.Grant, error) {
	c, err := wire.FromProto(capabilities, g.GetCapability())
	if err != nil {
		return org.Grant{}, err
	}
	m, err := wire.FromProto(grantModes, g.GetMode())
	if err != nil {
		return org.Grant{}, err
	}
	return org.Grant{Capability: c, Mode: m, Folder: g.GetFolder()}, nil
}

// grantsToProto is the bot's grants as the Access sheet lists them: the
// capabilities in the sheet's order, each folder's grant beneath its
// capability, and a capability the bot holds no grant for as one row, not
// granted, so the sheet shows every switch. own is the bot's own folder.
func grantsToProto(grants []org.Grant, own string) []*hearthv1.Grant {
	var out []*hearthv1.Grant
	for _, row := range capabilityRows {
		c := capabilities[row.capability]
		row := &hearthv1.Grant{
			Capability: row.capability, Mode: hearthv1.GrantMode_GRANT_MODE_NOT_GRANTED,
			Title: row.title, CanAllow: inbox.Standing(c),
		}
		n := len(out)
		for _, g := range grants {
			if g.Capability == c {
				one := &hearthv1.Grant{Capability: row.Capability, Title: row.Title, CanAllow: row.CanAllow, Folder: g.Folder}
				one.Mode = wire.ToProto(grantModes, g.Mode)
				one.OwnFolder = g.Folder != "" && g.Folder == own
				out = append(out, one)
			}
		}
		if len(out) == n {
			out = append(out, row)
		}
	}
	return out
}

// grantsOf is the bot's grants in the contract's form. The own folder is
// read from the bot; a bot that cannot be read has none marked.
func (o Deps) grantsOf(ctx context.Context, botID string, grants []org.Grant) []*hearthv1.Grant {
	bot, _ := o.Store.Bot(ctx, botID)
	return grantsToProto(grants, bot.FolderPath)
}

// itemToProto is an Inbox item. A failure item holds no conversation, so
// its link to the thread goes through its run.
func (o Deps) itemToProto(ctx context.Context, it inbox.Item) *hearthv1.InboxItem {
	out := &hearthv1.InboxItem{
		Id:               it.ID,
		Kind:             wire.ToProto(inboxKinds, it.Kind),
		State:            wire.ToProto(inboxStates, it.State),
		BotId:            it.BotID,
		ConversationId:   it.ConversationID,
		RunId:            it.RunID,
		MessageId:        it.MessageID,
		Title:            it.Title,
		Effect:           it.Effect,
		Question:         it.Question,
		SuggestedAnswers: it.Suggested,
		CreatedAt:        wire.Timestamp(it.CreatedAt),
		ClosedAt:         wire.Timestamp(it.ClosedAt),
		Decision:         wire.ToProto(decisions, it.Decision),
		Answer:           it.Answer,
		Outcome:          it.Outcome,
	}
	if it.Call != nil {
		out.Call = &hearthv1.ToolCall{CallId: it.Call.ID, Name: it.Call.Name, ArgumentsJson: it.Call.Arguments, Summary: it.Call.Summary}
	}
	control := wire.ControlOf(it.Widget)
	out.Widget, out.AnswerValue = control.Proto(), control.AnswerProto(it.WidgetAnswer)
	if out.ConversationId == "" && it.RunID != "" {
		if r, err := o.Store.Run(ctx, it.RunID); err == nil {
			out.ConversationId = r.ConversationID
		}
	}
	return out
}
