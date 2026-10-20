package rpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org/inbox"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/widgetwire"
)

// inboxService is InboxService: what waits on the owner. The orchestrator
// owns every rule about an item; these only name the item and the decision.
// See docs/model.md#8-approvals.
type inboxService struct{ opts Options }

func (s *inboxService) ListInbox(ctx context.Context, req *connect.Request[hearthv1.ListInboxRequest]) (*connect.Response[hearthv1.ListInboxResponse], error) {
	items, err := s.opts.Store.InboxItems(ctx, !req.Msg.GetIncludeClosed())
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(&hearthv1.ListInboxResponse{Items: each(items, func(it inbox.Item) *hearthv1.InboxItem {
		return s.opts.itemToProto(ctx, it)
	})}), nil
}

func (s *inboxService) Decide(ctx context.Context, req *connect.Request[hearthv1.DecideRequest]) (*connect.Response[hearthv1.DecideResponse], error) {
	m := req.Msg
	if err := need("item_id", m.GetItemId()); err != nil {
		return nil, err
	}
	d, err := fromProto(decisions, m.GetDecision())
	if err != nil {
		return nil, err
	}
	it, run, err := s.opts.Orchestrator.Decide(ctx, m.GetItemId(), d, m.GetReason())
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(&hearthv1.DecideResponse{Item: s.opts.itemToProto(ctx, it), Run: runToProto(run)}), nil
}

// Answer replies to a question in words, to its control, or both. The
// control's answer is read against the question's own control, which names
// the kind of answer and whether a date carries a time.
func (s *inboxService) Answer(ctx context.Context, req *connect.Request[hearthv1.AnswerRequest]) (*connect.Response[hearthv1.AnswerResponse], error) {
	m := req.Msg
	if err := need("item_id", m.GetItemId()); err != nil {
		return nil, err
	}
	var control widgetwire.Control
	if m.GetValue() != nil {
		asked, err := s.opts.Store.InboxItem(ctx, m.GetItemId())
		if err != nil {
			return nil, fail(err)
		}
		control = widgetwire.Of(asked.Widget)
	}
	it, run, err := s.opts.Orchestrator.Answer(ctx, m.GetItemId(), m.GetAnswer(), control.ReadAnswer(m.GetValue()))
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(&hearthv1.AnswerResponse{Item: s.opts.itemToProto(ctx, it), Run: runToProto(run)}), nil
}

func (s *inboxService) Acknowledge(ctx context.Context, req *connect.Request[hearthv1.AcknowledgeRequest]) (*connect.Response[hearthv1.AcknowledgeResponse], error) {
	if err := need("item_id", req.Msg.GetItemId()); err != nil {
		return nil, err
	}
	it, err := s.opts.Orchestrator.Acknowledge(ctx, req.Msg.GetItemId())
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(&hearthv1.AcknowledgeResponse{Item: s.opts.itemToProto(ctx, it)}), nil
}
