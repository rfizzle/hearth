package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/orchestrate/toolset"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// historyPage is how many earlier messages a run's history is read from.
const historyPage = 200

// ask is what a run answers: the owner's message, or on a resume the app's
// note, which is not stored, and the run that parked.
type ask struct {
	msg    org.Message
	parked string
}

// run answers prompt with one turn of bot's and records how it ended. ctx
// ends when the run is cancelled; every write uses a context that does not,
// so what a stopped run wrote is still saved.
func (s *Service) run(ctx context.Context, bot org.Bot, r org.Run, prompt ask) {
	k := &sink{s: s, ctx: context.WithoutCancel(ctx), run: r, botID: bot.ID, botName: bot.Name,
		calls: map[string]org.ToolCall{}, describe: map[string]func(json.RawMessage) string{}}
	k.run.State = org.RunWorking
	k.run.StartedAt = s.d.Now()
	k.saveRun()
	out, err := s.guarded(ctx, k, bot, prompt)
	if errors.Is(err, agent.ErrParked) && ctx.Err() != nil {
		// Cancel reached the run as it parked: the owner stopped it, so it
		// does not wait on them.
		err = agent.ErrCancelled
	}
	k.finish(out, err)
}

// guarded is the turn with a panic in it, the loop's or a tool's, turned
// into the error the run fails with: one bad tool call must not take the
// core down, nor leave the conversation busy for good. The panic is logged
// with its stack. A tool runs on a goroutine of its own, so its panic is
// caught there, stops the turn, and is reported here.
// See docs/model.md#9-failure-handling.
func (s *Service) guarded(ctx context.Context, k *sink, bot org.Bot, prompt ask) (out agent.Outcome, err error) {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	k.stop = stop
	defer func() {
		if p := recover(); p != nil {
			k.crash(p)
		}
		if p := k.crashed(); p != nil {
			out, err = agent.Outcome{}, fmt.Errorf("something went wrong inside hearth: %v", p)
		}
	}()
	return s.turn(ctx, k, bot, prompt)
}

// turn gathers what the bot's turn needs and runs it, reporting to k.
func (s *Service) turn(ctx context.Context, k *sink, bot org.Bot, prompt ask) (agent.Outcome, error) {
	o, err := s.d.Store.Org(k.ctx)
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
	history, text, err := s.history(k.ctx, prompt)
	if err != nil {
		return agent.Outcome{}, err
	}
	disp, err := s.dispatcher(k.ctx, bot, prompt.msg.ConversationID)
	if err != nil {
		return agent.Outcome{}, err
	}
	k.disp = disp
	var offered []agent.Tool
	for _, d := range disp.Tools() {
		offered = append(offered, agent.Tool{Name: d.Name, Description: d.Description, Parameters: d.Parameters})
		k.describe[d.Name] = d.Describe
	}
	stream, err := s.d.Streams(bot.Tier)
	if err != nil {
		return agent.Outcome{}, err
	}
	return agent.Run(ctx, agent.Input{
		System: framing.Render(framing.Input{
			OrgName: o.Name, BotName: bot.Name, Role: bot.Role,
			JobDescription: bot.JobDescription, FolderPath: folder,
		}),
		History: history,
		Prompt:  text,
		Tools:   offered,
		Stream:  stream,
		Execute: toolset.Execute(disp, k.crash),
		Gate:    disp.Gated,
		// See docs/model.md#4-termination-guarantees.
		MaxRounds: s.d.Caps.Steps,
		MaxWall:   s.d.Caps.Wall,
		Scrub:     s.d.Scrub,
		Sink:      k,
	})
}

// history is the conversation before prompt as the model reads it, and the
// text of the prompt. A resumed run is handed the round it parked in after
// the text before that round, and the app's note as its prompt.
// See docs/model.md#2-a-run.
func (s *Service) history(ctx context.Context, prompt ask) ([]agent.Message, string, error) {
	msgs, _, err := s.d.Store.Messages(ctx, prompt.msg.ConversationID, prompt.msg.Seq, historyPage)
	if err != nil {
		return nil, "", err
	}
	return s.d.Resume.History(msgs, prompt.parked, prompt.msg.Content)
}

// dispatcher is what checks bot's calls in the conversation convID: every
// tool configured now, the bot's grants and the conversation's allowances,
// read fresh so a decision counts from the next call.
func (s *Service) dispatcher(ctx context.Context, bot org.Bot, convID string) (tools.Dispatcher, error) {
	grants, err := s.d.Store.GrantsFor(ctx, bot.ID)
	if err != nil {
		return tools.Dispatcher{}, err
	}
	allowances, err := s.d.Store.Allowances(ctx, convID)
	if err != nil {
		return tools.Dispatcher{}, err
	}
	reg, err := s.registry()
	if err != nil {
		return tools.Dispatcher{}, err
	}
	return tools.Dispatcher{Defs: reg, Grants: grants, Allowances: allowances, Root: bot.FolderPath, Reserved: s.d.Reserved}, nil
}

// registry is every tool configured now: the file tools, asking the owner,
// web.fetch when there is a fetcher, and web.search when a search service
// has a key.
func (s *Service) registry() (*tools.Registry, error) {
	var searcher web.Searcher
	if s.d.Searcher != nil {
		if found, ok := s.d.Searcher(); ok {
			searcher = found
		}
	}
	return toolset.Registry(s.d.Fetcher, searcher, s.extra...)
}
