package park

import (
	"errors"
	"slices"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
)

// historyBudget is how much earlier text, in bytes, a run is given. The
// newest messages that fit are kept and older ones fall off.
const historyBudget = 48 << 10

// History is the conversation msgs, oldest first, as a run reads it before
// its prompt, and the prompt's text. A run that continues the parked run
// parked is handed the round it parked in after the text before that
// round, and prompt, the app's note, after it.
// See docs/model.md#2-a-run.
func (rs Resume) History(msgs []org.Message, parked, prompt string) ([]agent.Message, string, error) {
	if parked == "" {
		return textHistory(msgs), prompt, nil
	}
	round, ok := Find(msgs, parked)
	if !ok {
		return nil, "", errors.New("the round this run continues is no longer in the conversation")
	}
	before := slices.IndexFunc(msgs, func(m org.Message) bool { return m.Seq >= round.Start })
	history, text := rs.Assemble(textHistory(msgs[:before]), round, prompt)
	return history, text, nil
}

// textHistory is the newest text messages of msgs that fit the budget,
// oldest first. Tool rows are left out, and messages in a row from the same
// side are joined, since some providers refuse two turns in a row from one
// side.
func textHistory(msgs []org.Message) []agent.Message {
	var out []agent.Message
	used := 0
	for _, m := range slices.Backward(msgs) {
		if m.Kind != org.MessageText || m.Content == "" {
			continue
		}
		if used += len(m.Content); used > historyBudget {
			break
		}
		out = before(out, m)
	}
	slices.Reverse(out)
	return out
}

// before adds m to newest, the history read back from its end, joining it
// to the message it comes before when both are from the same side.
func before(newest []agent.Message, m org.Message) []agent.Message {
	role := agent.RoleBot
	if m.AuthorBotID == "" {
		role = agent.RoleOwner
	}
	if n := len(newest); n > 0 && newest[n-1].Role == role {
		newest[n-1].Text = m.Content + "\n\n" + newest[n-1].Text
		return newest
	}
	return append(newest, agent.Message{Role: role, Text: m.Content})
}
