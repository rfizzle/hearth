package orchestrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/orchestrate/after"
	"github.com/rfizzle/hearth/internal/orchestrate/record"
	"github.com/rfizzle/hearth/internal/orchestrate/toolset"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/files"
	"github.com/rfizzle/hearth/internal/tools/web"
	"github.com/rfizzle/hearth/internal/workspace"
)

// ask is what a run answers: the owner's message, or on a resume the app's
// note, which is not stored, and the run that parked.
type ask struct {
	msg    org.Message
	parked string
}

// run answers prompt with one turn of bot's and records how it ended, then
// on a run that ended done does the work it leaves. ctx ends when the run
// is cancelled; every write uses a context that does not, so what a stopped
// run wrote is still saved.
func (s *Service) run(ctx context.Context, bot org.Bot, r org.Run, prompt ask) {
	host := record.Host{Store: s.d.Store, Bus: s.d.Bus, Log: s.d.Log, Now: s.d.Now, Release: s.release}
	if s.d.Meter != nil {
		host.Meter = s.d.Meter
	}
	k := record.New(host, context.WithoutCancel(ctx), r, bot.ID, bot.Name)
	k.Start()
	var built assembly.Output
	out, err := s.guarded(ctx, k, bot, prompt, &built)
	if errors.Is(err, agent.ErrParked) && ctx.Err() != nil {
		// Cancel reached the run as it parked: the owner stopped it, so it
		// does not wait on them.
		err = agent.ErrCancelled
	}
	if reason := s.takeSpent(r.ID); reason != "" && (errors.Is(err, agent.ErrCancelled) || errors.Is(err, agent.ErrParked)) {
		// The budget stopped it, not the owner: it failed, and says why.
		err = errors.New(reason)
	}
	k.Finish(out, err)
	if k.Run().State == org.RunDone {
		s.afterwork.Run(context.WithoutCancel(ctx), s.afterInput(k), bot, built)
	}
}

// afterInput is what the work after the run k is handed.
func (s *Service) afterInput(k *record.Sink) after.Input {
	return after.Input{
		Store: s.d.Store, Bus: s.d.Bus, Log: s.d.Log, Streams: s.d.Streams, Models: s.d.Models, Scrub: s.d.Scrub,
		Run: k.Run(), Usage: k.Usage,
		Reserve: s.reserve, Release: func(convID string) { s.release(convID, "") },
	}
}

// takeSpent is the reason the budget stopped run runID, or "".
func (s *Service) takeSpent(runID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason := s.spent[runID]
	delete(s.spent, runID)
	return reason
}

// guarded is the turn with a panic in it, the loop's or a tool's, turned
// into the error the run fails with: one bad tool call must not take the
// core down, nor leave the conversation busy for good. The panic is logged
// with its stack. A tool runs on a goroutine of its own, so its panic is
// caught there, stops the turn, and is reported here.
// See docs/model.md#9-failure-handling.
func (s *Service) guarded(ctx context.Context, k *record.Sink, bot org.Bot, prompt ask, built *assembly.Output) (out agent.Outcome, err error) {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	k.SetStop(stop)
	defer func() {
		if p := recover(); p != nil {
			k.Crash(p)
		}
		if p := k.Crashed(); p != nil {
			out, err = agent.Outcome{}, fmt.Errorf("something went wrong inside hearth: %v", p)
		}
	}()
	return s.turn(ctx, k, bot, prompt, built)
}

// turn gathers what the bot's turn needs and runs it, reporting to k. The
// context it built is left in built.
func (s *Service) turn(ctx context.Context, k *record.Sink, bot org.Bot, prompt ask, built *assembly.Output) (agent.Outcome, error) {
	wctx := context.WithoutCancel(ctx) // for reads; a stop does not cancel it
	o, err := s.d.Store.Org(wctx)
	if err != nil {
		return agent.Outcome{}, err
	}
	folder := bot.FolderPath
	if !filepath.IsAbs(folder) {
		return agent.Outcome{}, fmt.Errorf("%s has no folder to work in", bot.Name)
	}
	// The dispatcher resolves each granted folder on disk and refuses a
	// call into one that does not exist, so a bot's first write would read
	// as a path outside its folders.
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return agent.Outcome{}, fmt.Errorf("could not create %s's folder: %w", bot.Name, err)
	}
	conv, err := s.d.Store.Conversation(wctx, prompt.msg.ConversationID)
	if err != nil {
		return agent.Outcome{}, err
	}
	proj, err := s.projectOf(wctx, o, conv)
	if err != nil {
		return agent.Outcome{}, err
	}
	// See docs/model.md#2-a-run.
	*built, err = assembly.Build(wctx, s.d.Store, assembly.Input{
		Org: o, Bot: bot, Conversation: conv, Prompt: prompt.msg,
		Parked: prompt.parked, Resume: s.d.Resume, Tier: bot.Tier, Project: proj,
	})
	if err != nil {
		return agent.Outcome{}, err
	}
	disp, err := s.dispatcher(wctx, bot, prompt.msg.ConversationID, prompt.msg.ID, proj)
	if err != nil {
		return agent.Outcome{}, err
	}
	k.Equip(disp)
	var offered []agent.Tool
	for _, d := range disp.Tools() {
		offered = append(offered, agent.Tool{Name: d.Name, Description: d.Description, Parameters: d.Parameters})
	}
	stream, err := s.d.Streams(bot)
	if err != nil {
		return agent.Outcome{}, err
	}
	var model agent.Model
	if s.d.Models != nil {
		model = s.d.Models(bot)
	}
	return agent.Run(ctx, agent.Input{
		Model:   model,
		System:  built.System,
		History: built.History,
		Prompt:  built.Prompt,
		Tools:   offered,
		Stream:  stream,
		Execute: toolset.Execute(disp, k.Crash),
		Gate:    disp.Gated,
		// See docs/model.md#4-termination-guarantees.
		MaxRounds: s.d.Caps.Steps,
		MaxWall:   s.d.Caps.Wall,
		Scrub:     s.d.Scrub,
		Sink:      k,
	})
}

// projectOf is the project conv is the conversation of, with its folder
// made, or nil for any other conversation.
func (s *Service) projectOf(ctx context.Context, o org.Org, conv org.Conversation) (*project.Project, error) {
	if conv.Kind != org.ConversationProject {
		return nil, nil
	}
	p, err := s.d.Store.Project(ctx, conv.ProjectID)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(p.FolderPath) {
		return nil, fmt.Errorf("project %s has no folder to work in", p.Name)
	}
	// The root must exist before a folder is made under it.
	if err := workspace.Ensure(o.WorkspacePath); err != nil {
		return nil, err
	}
	if err := workspace.EnsureFolder(o.WorkspacePath, p.FolderPath); err != nil {
		return nil, fmt.Errorf("could not create the folder of project %s: %w", p.Name, err)
	}
	return &p, nil
}

// projectIn is the project of the conversation convID, or nil.
func (s *Service) projectIn(ctx context.Context, convID string) (*project.Project, error) {
	conv, err := s.d.Store.Conversation(ctx, convID)
	if err != nil {
		return nil, err
	}
	o, err := s.d.Store.Org(ctx)
	if err != nil {
		return nil, err
	}
	return s.projectOf(ctx, o, conv)
}

// projectGrants are the grants a run in the project folder has beside the
// bot's own: read and write there, in memory only, never stored. Deleting
// there stays as the bot's grants say.
// See docs/model.md#access.
func projectGrants(botID, folder string) []org.Grant {
	return []org.Grant{
		{BotID: botID, Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
		{BotID: botID, Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
	}
}

// proposeGrant is the grant the Chief of Staff has for a run to propose a
// project, in memory only, never stored and never listed: the role gives it,
// not the owner.
// See docs/model.md#access.
func proposeGrant(botID string) org.Grant {
	return org.Grant{BotID: botID, Capability: project.CanPropose, Mode: org.ModeAllowed}
}

// dispatcher is what checks bot's calls in the conversation convID, made
// in answer to the message msgID: every tool configured now, the bot's
// grants with the read on the Inbox every bot has, and the conversation's
// allowances, read fresh so a decision counts from the next call. In a
// project's conversation, proj, the folder is the root and the bot may read
// and write there. The coordinator may also propose a project.
func (s *Service) dispatcher(ctx context.Context, bot org.Bot, convID, msgID string, proj *project.Project) (tools.Dispatcher, error) {
	grants, err := s.d.Store.GrantsFor(ctx, bot.ID)
	if err != nil {
		return tools.Dispatcher{}, err
	}
	o, err := s.d.Store.Org(ctx)
	if err != nil {
		return tools.Dispatcher{}, err
	}
	if g, ok := assembly.InboxGrant(o, bot.ID); ok {
		grants = append(grants, g)
	}
	root := bot.FolderPath
	if proj != nil {
		grants = append(grants, projectGrants(bot.ID, proj.FolderPath)...)
		root = proj.FolderPath
	}
	allowances, err := s.d.Store.Allowances(ctx, convID)
	if err != nil {
		return tools.Dispatcher{}, err
	}
	extra := []tools.Definition{toolset.Remember(s.d.Store, s.d.Bus, bot.ID, convID, msgID)}
	if c, ok := s.Coordinator(ctx); ok && c.ID == bot.ID {
		grants = append(grants, proposeGrant(bot.ID))
		extra = append(extra, toolset.Propose(s.d.Store, s.d.Bus, s.status, bot.ID, convID))
	}
	if s.d.Measures != nil {
		projectID := ""
		if proj != nil {
			projectID = proj.ID
		}
		extra = append(extra, toolset.Deliverables(s.d.Templates, s.d.Measures, bot.ID, projectID, s.d.Reserved))
	}
	reg, err := s.registry(append(extra, s.extra...))
	if err != nil {
		return tools.Dispatcher{}, err
	}
	return tools.Dispatcher{Defs: reg, Grants: grants, Allowances: allowances, Root: root, Reserved: s.d.Reserved}, nil
}

// registry is every tool configured now: the file tools, with the app's
// recogniser for this run when there is one, asking the owner, web.fetch when
// there is a fetcher, web.search when a search service has a key, then extra,
// the tools of this run alone.
func (s *Service) registry(extra []tools.Definition) (*tools.Registry, error) {
	var searcher web.Searcher
	if s.d.Searcher != nil {
		if found, ok := s.d.Searcher(); ok {
			searcher = found
		}
	}
	opts := files.Options{Reserved: s.d.Reserved}
	if s.d.Recogniser != nil {
		opts.Recogniser = s.d.Recogniser()
	}
	return toolset.Registry(opts, s.d.Fetcher.ForRun(), searcher, extra...)
}
