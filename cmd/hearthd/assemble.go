package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/models"
	"github.com/rfizzle/hearth/internal/rpc"
	"github.com/rfizzle/hearth/internal/skill"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools/web"
	"github.com/rfizzle/hearth/internal/workspace"
	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/pricing"
)

// openCore opens everything the contract serves over the data folder: the
// store, the accounts and the integrations over one key store, the fetcher
// and the orchestrator. The bus's heartbeats run until ctx ends. The
// returned function closes the store.
func openCore(ctx context.Context, dataDir string) (rpc.Options, func(), error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return rpc.Options{}, nil, err
	}
	reserved := reservedFolders(dataDir)
	provider.Init()
	db, err := store.Open(filepath.Join(dataDir, "hearth.db"))
	if err != nil {
		return rpc.Options{}, nil, err
	}
	accounts, err := provider.Open(dataDir, nil)
	if err != nil {
		_ = db.Close()
		return rpc.Options{}, nil, err
	}
	for _, c := range []org.Capability{org.AskOwner, org.Remember} {
		if _, err := db.BackfillAlwaysOn(ctx, c); err != nil {
			_ = db.Close()
			return rpc.Options{}, nil, fmt.Errorf("give older bots the right to %s: %w", c, err)
		}
	}
	integrations := &integration.Integrations{Keys: accounts.Keys}
	bus := orchestrate.NewBus()
	go bus.Heartbeats(ctx)
	// The price table is the snapshot inside the binary, or a newer one the
	// last refresh cached; an unreadable cache leaves the snapshot.
	table, err := pricing.Load()
	if err != nil {
		table = pricing.Snapshot()
	}
	// The meter stops a run through the service, which exists after it.
	var svc *orchestrate.Service
	meterOf := meter.New(db, table, nil,
		func(it inbox.Item) { bus.Publish(orchestrate.InboxChanged{Item: it}) },
		func(runID, reason string) {
			if svc != nil {
				svc.StopForBudget(runID, reason)
			}
		})
	deps := withModels(orchestrate.Deps{
		Store:   db,
		Meter:   meterOf,
		Fetcher: web.NewFetcher(),
		Searcher: func() (web.Searcher, bool) {
			s := integrations.Searcher()
			return s, s != nil
		},
		Bus: bus, Reserved: reserved,
	}, accounts)
	svc, err = orchestrate.Open(ctx, deps)
	if err != nil {
		_ = db.Close()
		return rpc.Options{}, nil, fmt.Errorf("open the orchestrator: %w", err)
	}
	return rpc.Options{
		Store:           db,
		Orchestrator:    svc,
		Accounts:        accounts,
		Integrations:    integrations,
		Templates:       skill.Templates(),
		ReservedFolders: reserved,
		Drafter:         drafter(skill.Templates(), deps.Streams, deps.Scrub),
		Bus:             bus,
		EnsureWorkspace: workspace.Ensure,
	}, func() { _ = db.Close() }, nil
}

// reservedFolders are the folders the tools and the grants must keep away
// from: the data folder, as given and as it resolves, since the app may be
// handed either.
func reservedFolders(dir string) []string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	out := []string{abs}
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != abs {
		out = append(out, real)
	}
	return out
}

// drafter proposes a bot with the model the Fast tier resolves to. A model
// that cannot be opened, or that fails, is unavailable to the app, in the
// words the owner reads; a reply that cannot be read is not.
func drafter(templates []org.RoleTemplate, streams func(org.Bot) (agent.Stream, error), scrub agent.Scrub) rpc.Drafter {
	return func(ctx context.Context, r rpc.DraftRequest) (rpc.Draft, error) {
		stream, err := streams(org.Bot{Tier: org.TierFast})
		if err != nil {
			return rpc.Draft{}, rpc.Unavailable(fmt.Errorf("%w: %s", draft.ErrUnavailable, agent.Explain(err)))
		}
		d, err := draft.Draft(ctx, draft.Input{
			Description: r.Description, TemplateID: r.TemplateID, Tier: r.Tier,
			Templates: templates, Stream: stream, Scrub: scrub,
		})
		if errors.Is(err, draft.ErrUnavailable) {
			err = rpc.Unavailable(err)
		}
		return rpc.Draft{Name: d.Name, Role: d.Role, JobDescription: d.JobDescription, WorkingStyle: d.WorkingStyle, Tier: d.Tier}, err
	}
}

// withModels gives the orchestrator its models and its scrub from the
// accounts: the stream for a bot, by its override or its tier, and the
// function that redacts every stored secret from what the model is sent. It
// is the one place the run depends on how the accounts resolve and scrub.
func withModels(d orchestrate.Deps, accounts *provider.Accounts) orchestrate.Deps {
	// The mapping is read on each call, so an edit in Settings reaches the
	// next run. A mapping that cannot be read leaves the defaults.
	resolve := func(b org.Bot) (provider.Model, error) {
		var rows []models.Row
		if saved, err := d.Store.Tiers(context.Background()); err == nil {
			for _, c := range saved {
				rows = append(rows, models.Row{Tier: provider.Tier(c.Tier), Pick: provider.Pick{Provider: provider.Provider(c.Provider), Profile: c.Profile, Model: c.Model}})
			}
		}
		return models.Resolve(accounts, models.Choice{Tier: provider.Tier(b.Tier), Override: b.ModelOverride}, rows)
	}
	d.Streams = func(b org.Bot) (agent.Stream, error) {
		m, err := resolve(b)
		if err != nil {
			return nil, err
		}
		return provider.StreamFunc(m), nil
	}
	d.Models = func(b org.Bot) agent.Model {
		m, err := resolve(b)
		if err != nil {
			return agent.Model{}
		}
		return agent.Model{Provider: string(m.Account), Name: m.Name}
	}
	scrub := accounts.Scrub()
	d.Scrub = func(m agent.Message) agent.Message {
		m.Text = scrub(llm.Message{Content: m.Text}).Content
		return m
	}
	return d
}
