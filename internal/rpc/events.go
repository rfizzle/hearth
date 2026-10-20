package rpc

import (
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The bodies rpc publishes on the bus beside orchestrate's: a change to an
// account, an integration or a bot is not a run, so it is rpc's to name.

// AccountChanged is a model provider's account in its new state.
type AccountChanged struct{ Account provider.Account }

// IntegrationChanged is an integration's account in its new state.
type IntegrationChanged struct{ Integration integration.Account }

// BotChanged is a bot as it is now: new, or changed.
type BotChanged struct{ Bot org.Bot }

// eventToProto is the SubscribeResponse for a bus event, and false for a body
// the contract has no message for.
func eventToProto(e orchestrate.Event) (*hearthv1.SubscribeResponse, bool) {
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
	case orchestrate.Resync:
		out.Event = &hearthv1.SubscribeResponse_Resync{Resync: &hearthv1.Resync{}}
	case orchestrate.Heartbeat:
		out.Event = &hearthv1.SubscribeResponse_Heartbeat{Heartbeat: &hearthv1.Heartbeat{}}
	case AccountChanged:
		out.Event = &hearthv1.SubscribeResponse_AccountChanged{AccountChanged: &hearthv1.AccountChanged{Account: accountToProto(b.Account)}}
	case IntegrationChanged:
		changed := &hearthv1.IntegrationChanged{Integration: integrationAccountToProto(b.Integration)}
		out.Event = &hearthv1.SubscribeResponse_IntegrationChanged{IntegrationChanged: changed}
	case BotChanged:
		out.Event = &hearthv1.SubscribeResponse_BotChanged{BotChanged: &hearthv1.BotChanged{Bot: botToProto(b.Bot)}}
	default:
		return nil, false
	}
	return out, true
}
