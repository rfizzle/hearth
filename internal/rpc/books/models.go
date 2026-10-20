package books

import (
	"context"
	"slices"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/provider/models"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/store"
)

// ListKnownServices is the services hearth knows the address of, with
// whether each runtime on this Mac answered just now and the models it
// listed.
// See docs/model.md#providers-and-models
func (s Models) ListKnownServices(ctx context.Context, _ *connect.Request[hearthv1.ListKnownServicesRequest]) (*connect.Response[hearthv1.ListKnownServicesResponse], error) {
	found := models.Detect(ctx, s.Accounts)
	return connect.NewResponse(&hearthv1.ListKnownServicesResponse{Services: wire.Each(found, knownServiceToProto)}), nil
}

// ListServiceModels asks a service which models it serves, so the card can
// offer a picker before the key is saved. A saved service is asked by its
// id; any other by the address and key in the request, which are used for
// this call and kept nowhere.
// See docs/model.md#providers-and-models
func (s Models) ListServiceModels(ctx context.Context, req *connect.Request[hearthv1.ListServiceModelsRequest]) (*connect.Response[hearthv1.ListServiceModelsResponse], error) {
	m := req.Msg
	ids, err := s.Accounts.ServiceModels(ctx, m.GetProfile(), m.GetBaseUrl(), m.GetKey())
	return wire.Reply(&hearthv1.ListServiceModelsResponse{Models: ids}, err)
}

// Tiers is the owner's mapping and every model the accounts offer, the
// runtimes on this Mac among them.
func (s Models) Tiers(ctx context.Context, _ *connect.Request[hearthv1.TiersRequest]) (*connect.Response[hearthv1.TiersResponse], error) {
	tiers, err := s.Store.Tiers(ctx)
	if err != nil {
		return nil, wire.Fail(err)
	}
	offered := models.Options(s.Accounts, models.Detect(ctx, s.Accounts))
	return connect.NewResponse(&hearthv1.TiersResponse{Tiers: wire.Each(tiers, tierToProto), Options: wire.Each(offered, optionToProto)}), nil
}

// SetTiers replaces the mapping and tells the bus; a tier left out follows
// the defaults, and a tier named twice is not one mapping.
func (s Models) SetTiers(ctx context.Context, req *connect.Request[hearthv1.SetTiersRequest]) (*connect.Response[hearthv1.SetTiersResponse], error) {
	choices := make([]store.TierChoice, 0, len(req.Msg.GetTiers()))
	for _, c := range req.Msg.GetTiers() {
		one, err := tierFromProto(c)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(choices, func(had store.TierChoice) bool { return had.Tier == one.Tier }) {
			return nil, wire.Invalid("%s is named twice", c.GetTier())
		}
		choices = append(choices, one)
	}
	if err := s.Store.PutTiers(ctx, choices); err != nil {
		return nil, wire.Fail(err)
	}
	stored, err := s.Store.Tiers(ctx)
	if err != nil {
		return nil, wire.Fail(err)
	}
	s.Publish(TiersChanged{Tiers: stored})
	return connect.NewResponse(&hearthv1.SetTiersResponse{Tiers: wire.Each(stored, tierToProto)}), nil
}
