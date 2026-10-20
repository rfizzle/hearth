package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// This file holds the handlers the contract declares before the core can
// serve them: grants, one conversation with its latest run, retry, and the
// Inbox. Each answers unimplemented until its real handler replaces it; the
// real handlers live beside the other services and this file goes away with
// the last of them.

var errNotYet = connect.NewError(connect.CodeUnimplemented, errors.New("the core does not do this yet"))

// inboxService is InboxService: what waits on the owner.
type inboxService struct{ opts Options }

func (s *botService) ListGrants(context.Context, *connect.Request[hearthv1.ListGrantsRequest]) (*connect.Response[hearthv1.ListGrantsResponse], error) {
	return nil, errNotYet
}

func (s *botService) SetGrants(context.Context, *connect.Request[hearthv1.SetGrantsRequest]) (*connect.Response[hearthv1.SetGrantsResponse], error) {
	return nil, errNotYet
}

func (s *conversationService) GetConversation(context.Context, *connect.Request[hearthv1.GetConversationRequest]) (*connect.Response[hearthv1.GetConversationResponse], error) {
	return nil, errNotYet
}

func (s *conversationService) Retry(context.Context, *connect.Request[hearthv1.RetryRequest]) (*connect.Response[hearthv1.RetryResponse], error) {
	return nil, errNotYet
}

func (s *inboxService) ListInbox(context.Context, *connect.Request[hearthv1.ListInboxRequest]) (*connect.Response[hearthv1.ListInboxResponse], error) {
	return nil, errNotYet
}

func (s *inboxService) Decide(context.Context, *connect.Request[hearthv1.DecideRequest]) (*connect.Response[hearthv1.DecideResponse], error) {
	return nil, errNotYet
}

func (s *inboxService) Answer(context.Context, *connect.Request[hearthv1.AnswerRequest]) (*connect.Response[hearthv1.AnswerResponse], error) {
	return nil, errNotYet
}

func (s *inboxService) Acknowledge(context.Context, *connect.Request[hearthv1.AcknowledgeRequest]) (*connect.Response[hearthv1.AcknowledgeResponse], error) {
	return nil, errNotYet
}
