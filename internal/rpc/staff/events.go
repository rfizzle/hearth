package staff

import (
	"context"

	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/rpc/books"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// eventToProto is the SubscribeResponse for a bus event, and false for a body
// the contract has no message for.
func (o Deps) eventToProto(ctx context.Context, e orchestrate.Event) (*hearthv1.SubscribeResponse, bool) {
	out := &hearthv1.SubscribeResponse{Sequence: e.Seq}
	switch b := e.Body.(type) {
	case orchestrate.MessageDelta:
		out.Event = &hearthv1.SubscribeResponse_MessageDelta{MessageDelta: &hearthv1.MessageDelta{
			ConversationId: b.ConversationID, MessageId: b.MessageID, RunId: b.RunID, Text: b.Text,
		}}
	case orchestrate.MessageUpserted:
		out.Event = &hearthv1.SubscribeResponse_MessageUpserted{MessageUpserted: &hearthv1.MessageUpserted{Message: messageToProto(b.Message)}}
	case orchestrate.RunChanged:
		out.Event = &hearthv1.SubscribeResponse_RunChanged{RunChanged: &hearthv1.RunChanged{Run: runToProto(b.Run)}}
	case orchestrate.MessageDeleted:
		out.Event = &hearthv1.SubscribeResponse_MessageDeleted{MessageDeleted: &hearthv1.MessageDeleted{
			ConversationId: b.ConversationID, MessageId: b.MessageID,
		}}
	case orchestrate.InboxChanged:
		out.Event = &hearthv1.SubscribeResponse_InboxChanged{InboxChanged: &hearthv1.InboxChanged{Item: o.itemToProto(ctx, b.Item)}}
	case orchestrate.GrantsChanged:
		out.Event = &hearthv1.SubscribeResponse_GrantsChanged{GrantsChanged: &hearthv1.GrantsChanged{
			BotId: b.BotID, Grants: o.grantsOf(ctx, b.BotID, b.Grants),
		}}
	case orchestrate.ConversationChanged:
		out.Event = &hearthv1.SubscribeResponse_ConversationChanged{ConversationChanged: &hearthv1.ConversationChanged{
			Conversation: conversationToProto(b.Conversation),
		}}
	case orchestrate.ProjectChanged:
		out.Event = &hearthv1.SubscribeResponse_ProjectChanged{ProjectChanged: &hearthv1.ProjectChanged{Project: projectToProto(b.Project, b.Status)}}
	case orchestrate.Resync:
		out.Event = &hearthv1.SubscribeResponse_Resync{Resync: &hearthv1.Resync{}}
	case orchestrate.Heartbeat:
		out.Event = &hearthv1.SubscribeResponse_Heartbeat{Heartbeat: &hearthv1.Heartbeat{}}
	case wire.BotChanged:
		out.Event = &hearthv1.SubscribeResponse_BotChanged{BotChanged: &hearthv1.BotChanged{Bot: wire.BotToProto(b.Bot)}}
	default:
		return o.otherEvent(e)
	}
	return out, true
}

// otherEvent is the SubscribeResponse for a body Deps.Events knows (the
// accounts' bodies), or one books publishes, or for a note the orchestrator
// does.
func (o Deps) otherEvent(e orchestrate.Event) (*hearthv1.SubscribeResponse, bool) {
	var out *hearthv1.SubscribeResponse
	ok := false
	if o.Events != nil {
		out, ok = o.Events(e.Body)
	}
	if !ok {
		out, ok = books.Event(e.Body)
	}
	if ok {
		out.Sequence = e.Seq
	}
	return out, ok
}
