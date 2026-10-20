package books

import (
	"cmp"
	"context"
	"strings"
	"unicode"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// ListPages is the pages a bot reads, everyone's first, and how full they
// make its handbook; with no bot, the pages for everyone.
// See docs/model.md#knowledge
func (s Handbook) ListPages(ctx context.Context, req *connect.Request[hearthv1.ListPagesRequest]) (*connect.Response[hearthv1.ListPagesResponse], error) {
	pages, fill, err := s.Pages.List(ctx, req.Msg.GetBotId())
	return wire.Reply(&hearthv1.ListPagesResponse{Pages: wire.Each(pages, pageToProto), Meter: meterToProto(fill)}, err)
}

// PutPage adds or replaces a page; the handbook package refuses one with no
// title or one that overfills a reader. A title is one line of words. The meter is the page's reader's:
// the bot it is for, or everyone. The handbook tells the bus.
func (s Handbook) PutPage(ctx context.Context, req *connect.Request[hearthv1.PutPageRequest]) (*connect.Response[hearthv1.PutPageResponse], error) {
	m, scope := req.Msg, memory.Scope{Everyone: req.Msg.GetScope().GetEveryone(), BotID: req.Msg.GetScope().GetBotId()}
	if err := wire.Words("the title", m.GetTitle()); err != nil {
		return nil, wire.Fail(err)
	}
	if strings.ContainsFunc(m.GetTitle(), unicode.IsControl) {
		return nil, wire.Invalid("the title is one line; write it without a line break or a tab")
	}
	page, err := s.Pages.Put(ctx, memory.Page{ID: m.GetId(), Title: m.GetTitle(), Markdown: m.GetMarkdown(), Scope: scope})
	if err != nil {
		return nil, wire.Fail(err)
	}
	_, fill, err := s.Pages.List(ctx, page.Scope.BotID)
	return wire.Reply(&hearthv1.PutPageResponse{Page: pageToProto(page), Meter: meterToProto(fill)}, err)
}

// DeletePage removes a page; the meter is everyone's, since the request
// names no reader. The handbook tells the bus.
func (s Handbook) DeletePage(ctx context.Context, req *connect.Request[hearthv1.DeletePageRequest]) (*connect.Response[hearthv1.DeletePageResponse], error) {
	if err := wire.Need("id", req.Msg.GetId()); err != nil {
		return nil, err
	}
	if err := s.Pages.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, wire.Fail(err)
	}
	_, fill, err := s.Pages.List(ctx, "")
	return wire.Reply(&hearthv1.DeletePageResponse{Meter: meterToProto(fill)}, err)
}

// Summary is a month's cost; an empty month is this one.
// See docs/model.md#usage
func (s Usage) Summary(ctx context.Context, req *connect.Request[hearthv1.SummaryRequest]) (*connect.Response[hearthv1.SummaryResponse], error) {
	sum, err := s.Meter.Summary(ctx, req.Msg.GetMonth())
	return wire.Reply(summaryToProto(sum), err)
}

// SetBudgets sets the org's budget and each bot's named; the meter decides
// what a new limit does to a pause. The answer is this month's summary.
func (s Usage) SetBudgets(ctx context.Context, req *connect.Request[hearthv1.SetBudgetsRequest]) (*connect.Response[hearthv1.SetBudgetsResponse], error) {
	list := []usage.Budget{{Subject: usage.Org, Cents: req.Msg.GetOrgCents()}}
	for _, b := range req.Msg.GetBots() {
		if err := wire.Need("bot_id", b.GetBotId()); err != nil {
			return nil, err
		}
		list = append(list, usage.Budget{Subject: b.GetBotId(), Cents: b.GetCents()})
	}
	sum, err := s.thisMonth(ctx, s.Meter.SetBudgets(ctx, list))
	return wire.Reply(&hearthv1.SetBudgetsResponse{Summary: sum}, err)
}

// Resume lets a paused bot, or with no bot the org, work again this month.
func (s Usage) Resume(ctx context.Context, req *connect.Request[hearthv1.ResumeRequest]) (*connect.Response[hearthv1.ResumeResponse], error) {
	subject := cmp.Or(req.Msg.GetBotId(), usage.Org)
	sum, err := s.thisMonth(ctx, s.Meter.Resume(ctx, subject))
	return wire.Reply(&hearthv1.ResumeResponse{Summary: sum}, err)
}

// thisMonth is this month's summary after a change that ended in err.
func (s Usage) thisMonth(ctx context.Context, err error) (*hearthv1.SummaryResponse, error) {
	if err != nil {
		return nil, err
	}
	sum, err := s.Meter.Summary(ctx, "")
	return summaryToProto(sum), err
}
