package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/document"
	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/models"
	"github.com/rfizzle/hearth/internal/rpc"
	"github.com/rfizzle/hearth/internal/rpc/books"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/skill"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools/web"
	"github.com/rfizzle/hearth/internal/workspace"
	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/pricing"
)

// openCore opens everything the contract serves over the data folder: the
// store, the accounts and the integrations over one key store, the fetcher,
// the meter, the handbook and the orchestrator, which is handed recogniser,
// the app's reader of pictures for each run, or nil without the app. The
// bus's heartbeats run until ctx ends. The returned function closes the
// store.
func openCore(ctx context.Context, dataDir string, recogniser func() document.Recogniser) (rpc.Options, func(), error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return rpc.Options{}, nil, err
	}
	provider.Init()
	db, err := store.Open(filepath.Join(dataDir, "hearth.db"))
	if err != nil {
		return rpc.Options{}, nil, err
	}
	reserved := reservedList(db, dataDir)
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
	if err := hireChiefOfStaff(ctx, db); err != nil {
		_ = db.Close()
		return rpc.Options{}, nil, fmt.Errorf("hire the chief of staff: %w", err)
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
	// The meter stops a run through the service, which exists after it, and
	// its ticks read its own summary, so both are bound once it is made.
	var svc *orchestrate.Service
	var tick func(meter.Tick)
	meterOf := meter.New(db, table, func(t meter.Tick) { tick(t) },
		func(it inbox.Item) { bus.Publish(orchestrate.InboxChanged{Item: it}) },
		func(runID, reason string) {
			if svc != nil {
				svc.StopForBudget(runID, reason)
			}
		})
	tick = books.Ticks(meterOf, func(body any) { bus.Publish(body) })
	pages := &handbook.Service{Store: db, OnChange: func(p memory.Page, deleted bool) {
		bus.Publish(books.PageChanged{Page: p, Deleted: deleted})
	}}
	// The project rule is the orchestrator's, made below; a figure is read
	// only once a run is under way, after it exists.
	figures := measures(db, func(ctx context.Context, id string) (project.Status, error) {
		return svc.Status(ctx, id)
	}, meterOf, pages)
	deps := withModels(orchestrate.Deps{
		Store:   db,
		Meter:   meterOf,
		Fetcher: web.NewFetcher(),
		Searcher: func() (web.Searcher, bool) {
			s := integrations.Searcher()
			return s, s != nil
		},
		Recogniser: recogniser,
		Bus:        bus, Reserved: reserved,
		Templates: deliverableTemplates(db), Measures: figures,
		Coordinator: func(ctx context.Context) (org.Bot, bool) {
			b, err := db.Coordinator(ctx)
			return b, err == nil
		},
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
		ProjectDrafter:  projectDrafter(deps.Streams, deps.Scrub),
		Bus:             bus,
		EnsureWorkspace: workspace.Ensure,
		Handbook:        pages,
		Meter:           meterOf,
		Deliverables:    deliverableDeps(db, figures),
	}, func() { _ = db.Close() }, nil
}

// hireChiefOfStaff gives an org named before the Chief of Staff came with
// onboarding the one it would have had, so an existing data folder gains it
// on the next launch. Before onboarding there is no org and nothing to do;
// an org that has an org-level bot keeps it, and one whose team already
// holds the folder the Chief of Staff would take is left without one.
// See docs/model.md#5-the-chief-of-staffs-routing-policy
func hireChiefOfStaff(ctx context.Context, db *store.DB) error {
	o, found, err := db.FindOrg(ctx)
	if err != nil || !found {
		return err
	}
	bot, grants, dm := skill.ChiefOfStaff(o.WorkspacePath)
	_, _, err = db.PutOrgWithCoordinator(ctx, o, bot, grants, dm)
	return err
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

// reservedList is asked at every check: the data folder, and the owner's
// templates in the Hearth folder once the org names one, so a folder named
// while the core runs is reserved at once. The org is one row.
func reservedList(db *store.DB, dataDir string) workspace.Reserved {
	data := reservedFolders(dataDir)
	return func() []string {
		o, found, err := db.FindOrg(context.Background())
		if err != nil || !found || o.WorkspacePath == "" {
			return data
		}
		return append(slices.Clone(data), templatesFolders(o.WorkspacePath)...)
	}
}

// drafter proposes a bot with the model the Fast tier resolves to. A model
// that cannot be opened, or that fails, is unavailable to the app, in the
// words the owner reads; a reply that cannot be read is not.
func drafter(templates []org.RoleTemplate, streams func(org.Bot) (agent.Stream, error), scrub agent.Scrub) rpc.Drafter {
	return func(ctx context.Context, r rpc.DraftRequest) (rpc.Draft, error) {
		stream, err := fastStream(streams)
		if err != nil {
			return rpc.Draft{}, err
		}
		d, err := draft.Draft(ctx, draft.Input{
			Description: r.Description, TemplateID: r.TemplateID, Tier: r.Tier,
			Templates: templates, Stream: stream, Scrub: scrub,
		})
		if errors.Is(err, draft.ErrUnavailable) {
			err = wire.Unavailable(err)
		}
		return rpc.Draft{Name: d.Name, Role: d.Role, JobDescription: d.JobDescription, WorkingStyle: d.WorkingStyle, Tier: d.Tier, Concerns: d.Concerns}, err
	}
}

// fastStream opens the model the Fast tier resolves to, or says in the words
// the owner reads that none can be.
func fastStream(streams func(org.Bot) (agent.Stream, error)) (agent.Stream, error) {
	stream, err := streams(org.Bot{Tier: org.TierFast})
	if err != nil {
		return nil, wire.Unavailable(fmt.Errorf("%w: %s", draft.ErrUnavailable, agent.Explain(err)))
	}
	return stream, nil
}

// projectDrafter proposes a project from the owner's paragraph with the model
// the Fast tier resolves to, unavailable and malformed as drafter is. The
// paragraph is the owner's, bounded by the handler, and goes to the model
// scrubbed and as data; nothing in the reply is used beyond the proposal.
func projectDrafter(streams func(org.Bot) (agent.Stream, error), scrub agent.Scrub) rpc.ProjectDrafter {
	return func(ctx context.Context, description string) (rpc.ProjectDraft, error) {
		stream, err := fastStream(streams)
		if err != nil {
			return rpc.ProjectDraft{}, err
		}
		d, err := draft.DraftProject(ctx, draft.ProjectInput{Description: description, Stream: stream, Scrub: scrub})
		if errors.Is(err, draft.ErrUnavailable) {
			err = wire.Unavailable(err)
		}
		return rpc.ProjectDraft{Name: d.Name, Brief: d.Brief, TargetDate: d.TargetDate, Concerns: d.Concerns}, err
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
