package assembly

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
)

// summaryInputCap is the most text, in bytes, one summary folds in; what is
// left is folded in after the next run.
const summaryInputCap = 64 << 10

// Summarise asks the model, once and with no tools, to fold the text of the
// conversation conversationID that the summary does not cover yet and that
// comes before until into the summary, oldest first and at most 64 KiB of
// it, and stores the result with the seq of the last message it folded in.
// It is called after a run whose window left text out; nothing to fold in
// changes nothing. A failure changes nothing either, and the next run that
// leaves text out tries again. The request's usage goes to side.Usage.
// See docs/model.md#2-a-run.
func Summarise(ctx context.Context, db *store.DB, side Side, conversationID string, until int64) error {
	c, err := db.Conversation(ctx, conversationID)
	if err != nil {
		return err
	}
	msgs, err := uncovered(ctx, db, conversationID, c.SummaryThroughSeq, until)
	if err != nil || len(msgs) == 0 {
		return err
	}
	name := "the bot"
	if b, err := db.Bot(ctx, c.BotID); err == nil {
		name = b.Name
	}
	prompt, through := summaryPrompt(c.Summary, msgs, name)
	// Owner- and bot-written text enters here: the summary so far and the
	// conversation's messages, under the labelled headings of the prompt,
	// which summary.md says are data.
	reply, err := side.ask(ctx, framing.RenderSummary(name), prompt)
	if err != nil {
		return fmt.Errorf("summarise: %s", agent.Explain(err))
	}
	text := strings.TrimSpace(reply)
	if text == "" {
		return errors.New("summarise: the model returned no summary")
	}
	return db.PutSummary(ctx, conversationID, text, through)
}

// uncovered is the text of the conversation after through and before until,
// oldest first.
func uncovered(ctx context.Context, db *store.DB, convID string, through, until int64) ([]org.Message, error) {
	var msgs []org.Message
	before := until
	for before > through+1 {
		got, more, err := db.Messages(ctx, convID, before, page)
		if err != nil {
			return nil, err
		}
		msgs = append(got, msgs...)
		if !more || len(got) == 0 {
			break
		}
		before = got[0].Seq
	}
	var out []org.Message
	for _, m := range msgs {
		if inWindow(m, through, until) {
			out = append(out, m)
		}
	}
	return out, nil
}

// summaryPrompt is the summary so far and msgs, oldest first, each under
// the name of who wrote it, as many as fit summaryInputCap, at least one
// even if cut; and the seq of the last one it holds.
func summaryPrompt(summary string, msgs []org.Message, botName string) (string, int64) {
	var b strings.Builder
	b.WriteString("The summary so far:\n\n")
	if s := strings.TrimSpace(summary); s != "" {
		b.WriteString(s)
	} else {
		b.WriteString("(none yet)")
	}
	b.WriteString("\n\nThe messages to fold in, oldest first:\n")
	var through int64
	used := 0
	for _, m := range msgs {
		who := "The person"
		if m.AuthorBotID != "" {
			who = botName
		}
		entry := "\n" + who + ": " + m.Content + "\n"
		if used+len(entry) > summaryInputCap {
			if through != 0 {
				break
			}
			entry = cut(entry, summaryInputCap) + "\n"
		}
		used += len(entry)
		through = m.Seq
		b.WriteString(entry)
	}
	return b.String(), through
}
