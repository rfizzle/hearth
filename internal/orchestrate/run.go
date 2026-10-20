package orchestrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/files"
	"github.com/rfizzle/hearth/internal/tools/web"
)

const (
	// historyBudget is how much earlier text, in bytes, a run is given. The
	// newest messages that fit are kept and older ones fall off.
	historyBudget = 48 << 10
	// historyPage is how many earlier messages are read to fill it.
	historyPage = 200
)

// run answers prompt with one turn of bot's and records how it ended. ctx
// ends when the run is cancelled; every write uses a context that does not,
// so what a stopped run wrote is still saved.
func (s *Service) run(ctx context.Context, bot org.Bot, r org.Run, prompt org.Message) {
	k := &sink{s: s, ctx: context.WithoutCancel(ctx), run: r, botID: bot.ID,
		calls: map[string]org.ToolCall{}, describe: map[string]func(json.RawMessage) string{}}
	k.run.State = org.RunWorking
	k.run.StartedAt = s.d.Now()
	k.saveRun()
	out, err := s.turn(ctx, k, bot, prompt)
	k.finish(out, err)
}

// turn gathers what the bot's turn needs and runs it, reporting to k.
func (s *Service) turn(ctx context.Context, k *sink, bot org.Bot, prompt org.Message) (agent.Outcome, error) {
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
	grants, err := s.d.Store.GrantsFor(k.ctx, bot.ID)
	if err != nil {
		return agent.Outcome{}, err
	}
	history, err := s.history(k.ctx, prompt)
	if err != nil {
		return agent.Outcome{}, err
	}
	reg, err := s.registry()
	if err != nil {
		return agent.Outcome{}, err
	}
	disp := tools.Dispatcher{Defs: reg, Grants: grants, Root: folder}
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
		Prompt:  prompt.Content,
		Tools:   offered,
		Stream:  stream,
		Execute: execute(disp),
		// See docs/model.md#4-termination-guarantees.
		MaxRounds: s.d.Caps.Steps,
		MaxWall:   s.d.Caps.Wall,
		Scrub:     s.d.Scrub,
		Sink:      k,
	})
}

// history is the conversation before prompt as the model reads it: the
// newest text messages that fit the budget, oldest first. Tool rows are
// left out, and messages in a row from the same side are joined, since some
// providers refuse two turns in a row from one side.
func (s *Service) history(ctx context.Context, prompt org.Message) ([]agent.Message, error) {
	msgs, _, err := s.d.Store.Messages(ctx, prompt.ConversationID, prompt.Seq, historyPage)
	if err != nil {
		return nil, err
	}
	var out []agent.Message
	used := 0
	for _, m := range slices.Backward(msgs) {
		if m.Kind != org.MessageText || m.Content == "" {
			continue
		}
		if used += len(m.Content); used > historyBudget {
			break
		}
		role := agent.RoleBot
		if m.AuthorBotID == "" {
			role = agent.RoleOwner
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Text = m.Content + "\n\n" + out[n-1].Text
			continue
		}
		out = append(out, agent.Message{Role: role, Text: m.Content})
	}
	slices.Reverse(out)
	return out, nil
}

// registry is every tool configured now: the file tools, web.fetch when
// there is a fetcher, and web.search when a search service has a key.
func (s *Service) registry() (*tools.Registry, error) {
	var searcher web.Searcher
	if s.d.Searcher != nil {
		if found, ok := s.d.Searcher(); ok {
			searcher = found
		}
	}
	return tools.NewRegistry(append(files.Definitions(), web.Definitions(s.d.Fetcher, searcher)...)...)
}

// execute runs a call through the dispatcher, the one place its grants are
// checked, and hands the loop the paths of the files it wrote.
func execute(d tools.Dispatcher) agent.Execute {
	return func(ctx context.Context, name string, args json.RawMessage) (agent.Result, error) {
		r, err := d.Execute(ctx, name, args)
		paths := make([]string, len(r.Files))
		for i, f := range r.Files {
			paths[i] = f.Path
		}
		return agent.Result{Text: r.Text, Files: paths}, err
	}
}
