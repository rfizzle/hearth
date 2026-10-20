package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/agent"
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
	integrations := &integration.Integrations{Keys: accounts.Keys}
	bus := orchestrate.NewBus()
	go bus.Heartbeats(ctx)
	svc, err := orchestrate.Open(ctx, withModels(orchestrate.Deps{
		Store:   db,
		Fetcher: web.NewFetcher(),
		Searcher: func() (web.Searcher, bool) {
			s := integrations.Searcher()
			return s, s != nil
		},
		Bus: bus,
	}, accounts))
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
		Bus:          bus,
	}, func() { _ = db.Close() }, nil
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
