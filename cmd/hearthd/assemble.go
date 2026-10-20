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
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/rpc"
	"github.com/rfizzle/hearth/internal/skill"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools/web"
	"github.com/rfizzle/llm"
)

// openCore opens everything the contract serves over the data folder: the
// store, the accounts and the integrations over one key store, the fetcher
// and the orchestrator. The bus's heartbeats run until ctx ends. The
// returned function closes the store.
func openCore(ctx context.Context, dataDir string) (rpc.Options, func(), error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return rpc.Options{}, nil, err
	}
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
	if _, err := db.BackfillAskOwner(ctx); err != nil {
		_ = db.Close()
		return rpc.Options{}, nil, fmt.Errorf("give older bots the right to ask: %w", err)
	}
	integrations := &integration.Integrations{Keys: accounts.Keys}
	bus := orchestrate.NewBus()
	go bus.Heartbeats(ctx)
	deps := withModels(orchestrate.Deps{
		Store:   db,
		Fetcher: web.NewFetcher(),
		Searcher: func() (web.Searcher, bool) {
			s := integrations.Searcher()
			return s, s != nil
		},
		Bus: bus,
	}, accounts)
	svc, err := orchestrate.Open(ctx, deps)
	if err != nil {
		_ = db.Close()
		return rpc.Options{}, nil, fmt.Errorf("open the orchestrator: %w", err)
	}
	return rpc.Options{
		Store:        db,
		Orchestrator: svc,
		Accounts:     accounts,
		Integrations: integrations,
		Templates:    skill.Templates(),
		Drafter:      drafter(skill.Templates(), deps.Streams, deps.Scrub),
		Bus:          bus,
	}, func() { _ = db.Close() }, nil
}

// drafter proposes a bot with the model the Fast tier resolves to. A model
// that cannot be opened, or that fails, is unavailable to the app, in the
// words the owner reads; a reply that cannot be read is not.
func drafter(templates []org.RoleTemplate, streams func(org.Tier) (agent.Stream, error), scrub agent.Scrub) rpc.Drafter {
	return func(ctx context.Context, r rpc.DraftRequest) (rpc.Draft, error) {
		stream, err := streams(org.TierFast)
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
// accounts: the stream for a bot's tier, and the function that redacts every
// stored secret from what the model is sent. It is the one place the run
// depends on how the accounts resolve and scrub.
func withModels(d orchestrate.Deps, accounts *provider.Accounts) orchestrate.Deps {
	d.Streams = func(t org.Tier) (agent.Stream, error) {
		m, err := accounts.Resolve(provider.Tier(t))
		if err != nil {
			return nil, err
		}
		return provider.StreamFunc(m), nil
	}
	scrub := accounts.Scrub()
	d.Scrub = func(m agent.Message) agent.Message {
		m.Text = scrub(llm.Message{Content: m.Text}).Content
		return m
	}
	return d
}
