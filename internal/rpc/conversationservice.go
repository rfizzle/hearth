package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

func (s *conversationService) ListConversations(ctx context.Context, req *connect.Request[hearthv1.ListConversationsRequest]) (*connect.Response[hearthv1.ListConversationsResponse], error) {
	botID := req.Msg.GetBotId()
	if err := wire.Need("bot_id", botID); err != nil {
		return nil, err
	}
	convs, err := s.opts.Store.Conversations(ctx, botID)
	return wire.Reply(&hearthv1.ListConversationsResponse{Conversations: wire.Each(convs, conversationToProto)}, err)
}

func (s *conversationService) ListMessages(ctx context.Context, req *connect.Request[hearthv1.ListMessagesRequest]) (*connect.Response[hearthv1.ListMessagesResponse], error) {
	m := req.Msg
	if err := wire.Need("conversation_id", m.GetConversationId()); err != nil {
		return nil, err
	}
	msgs, more, err := s.opts.Store.Messages(ctx, m.GetConversationId(), m.GetBeforeSeq(), int(m.GetLimit()))
	return wire.Reply(&hearthv1.ListMessagesResponse{Messages: wire.Each(msgs, messageToProto), HasMore: more}, err)
}

func (s *conversationService) Send(ctx context.Context, req *connect.Request[hearthv1.SendRequest]) (*connect.Response[hearthv1.SendResponse], error) {
	if err := wire.Need("conversation_id", req.Msg.GetConversationId(), "text", req.Msg.GetText()); err != nil {
		return nil, err
	}
	run, msg, err := s.opts.Orchestrator.Send(ctx, req.Msg.GetConversationId(), req.Msg.GetText(), req.Msg.GetAttachmentPaths())
	return wire.Reply(&hearthv1.SendResponse{Message: messageToProto(msg), Run: runToProto(run)}, err)
}

func (s *conversationService) Cancel(_ context.Context, req *connect.Request[hearthv1.CancelRequest]) (*connect.Response[hearthv1.CancelResponse], error) {
	if err := wire.Need("run_id", req.Msg.GetRunId()); err != nil {
		return nil, err
	}
	if !s.opts.Orchestrator.Cancel(req.Msg.GetRunId()) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such run is working"))
	}
	return connect.NewResponse(&hearthv1.CancelResponse{}), nil
}

// GetConversation is the conversation with its newest run, so a screen
// opened mid-run shows it.
func (s *conversationService) GetConversation(ctx context.Context, req *connect.Request[hearthv1.GetConversationRequest]) (*connect.Response[hearthv1.GetConversationResponse], error) {
	if err := wire.Need("id", req.Msg.GetId()); err != nil {
		return nil, err
	}
	c, err := s.opts.Store.Conversation(ctx, req.Msg.GetId())
	return wire.Reply(&hearthv1.GetConversationResponse{Conversation: conversationToProto(c)}, err)
}

// Retry runs the owner message behind a failed or cancelled run again. A run
// in any other state is a precondition failure, read from the run's row; the
// orchestrator takes a conversation, so the run is also the way to name it.
// See docs/model.md#9-failure-handling
func (s *conversationService) Retry(ctx context.Context, req *connect.Request[hearthv1.RetryRequest]) (*connect.Response[hearthv1.RetryResponse], error) {
	if err := wire.Need("run_id", req.Msg.GetRunId()); err != nil {
		return nil, err
	}
	was, err := s.opts.Store.Run(ctx, req.Msg.GetRunId())
	if err != nil {
		return nil, wire.Fail(err)
	}
	if was.State != org.RunFailed && was.State != org.RunCancelled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only a failed or cancelled run can be retried"))
	}
	run, err := s.opts.Orchestrator.Retry(ctx, was.ConversationID)
	return wire.Reply(&hearthv1.RetryResponse{Run: runToProto(run)}, err)
}
