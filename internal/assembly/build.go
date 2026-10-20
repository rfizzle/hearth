package assembly

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/store"
)

// The caps of the parts of a run's context, in bytes: the core has no
// tokenizer, and with them the whole stays under 128 KiB, which every model
// a tier maps to accepts. The handbook's allows for the page headings and
// the notice of pages left out, which the handbook's own cap does not count.
// See docs/model.md#2-a-run.
const (
	handbookCap = memory.HandbookCap + 4<<10
	notesCap    = 8 << 10
	summaryCap  = 4 << 10
	attachCap   = 2 << 10
	pendingCap  = 32 << 10
)

// page is how many messages one read of the thread takes.
const page = 200

// summaryLead begins the summary's entry in the history.
const summaryLead = "Earlier in this conversation:\n\n"

// window is how many bytes of the conversation a run on tier reads.
func window(t org.Tier) int {
	switch t {
	case org.TierFast:
		return 32 << 10
	case org.TierBest:
		return 64 << 10
	}
	return 48 << 10
}

// Input is what a run's context is built from. Prompt is the owner's
// message, or on a resume the app's note, which is not stored; Parked is
// the run a resumed run continues, and Resume how it is handed the round it
// parked in. Tier sets the conversation window.
type Input struct {
	Org          org.Org
	Bot          org.Bot
	Conversation org.Conversation
	Prompt       org.Message
	Parked       string
	Resume       Resume
	Tier         org.Tier
}

// Parts is how many bytes each part of a context took. Brief is a task's,
// which a direct conversation does not have.
type Parts struct {
	Framing, Handbook, Notes, Summary, Window, Brief, Attachments, Pending int
}

// Output is a run's context: the system text, the history before the
// prompt, the prompt, and what each part took. Behind reports that the
// window left out text the summary does not cover, and Until is the seq the
// window starts at: what Summarise brings the summary up to.
type Output struct {
	System  string
	History []agent.Message
	Prompt  string
	Parts   Parts
	Behind  bool
	Until   int64
}

// Build is the context a run of in.Bot answers in.Prompt in, in the order
// and within the caps of docs/model.md#2-a-run: the framing and the job
// description, the bot's handbook pages and its notes in the system text;
// the conversation's summary, then the newest messages after it that fit
// the tier's window, then on a resume the round the run parked in, in the
// history; and the prompt with the files attached to it.
//
// Text the owner, a page or the bot wrote enters only inside the labelled
// blocks the framing names: the handbook under "## Handbook", the notes
// under "## Your notes", the summary in an entry that begins "Earlier in
// this conversation:", and attachments as "File attached:" lines. Nothing
// read here is rendered as part of the framing.
func Build(ctx context.Context, db *store.DB, in Input) (Output, error) {
	var out Output
	// Owner-supplied text: the org's name and the bot's name, role and job
	// description, in the places framing's dm.md gives them.
	framed := framing.Render(framing.Input{
		OrgName: in.Org.Name, BotName: in.Bot.Name, Role: in.Bot.Role,
		JobDescription: in.Bot.JobDescription, FolderPath: in.Bot.FolderPath,
	})
	pages, err := db.Pages(ctx)
	if err != nil {
		return Output{}, err
	}
	notes, err := db.Notes(ctx, in.Bot.ID)
	if err != nil {
		return Output{}, err
	}
	book, kept := handbookBlock(memory.Select(pages, in.Bot.ID)), notesBlock(notes)
	out.Parts.Framing, out.Parts.Handbook, out.Parts.Notes = len(framed), len(book), len(kept)
	out.System = joinBlocks(framed, book, kept)

	msgs, round, bound, err := read(ctx, db, in)
	if err != nil {
		return Output{}, err
	}
	var hist []agent.Message
	hist, out.Parts.Window, out.Behind, out.Until = windowOf(msgs, in.Conversation.SummaryThroughSeq, bound, window(in.Tier))
	hist, out.Parts.Summary = withSummary(hist, in.Conversation.Summary)
	if in.Parked == "" {
		lines := attachLines(in.Prompt.Attachments)
		out.Parts.Attachments = len(lines)
		out.History, out.Prompt = hist, joinBlocks(in.Prompt.Content, lines)
		return out, nil
	}
	// Tool results (page text, file contents) enter here, as tool messages
	// answering the calls of the parked round, or written out in the app's
	// note with Resume.AsText.
	round = round.capped(pendingCap)
	out.Parts.Pending = round.size()
	out.History, out.Prompt = in.Resume.Assemble(hist, round, in.Prompt.Content)
	return out, nil
}

// read is the thread before the prompt, oldest first, read back a page at a
// time until it reaches what the summary covers, the start of the thread,
// or more text than the window holds; on a resume the round the run parked
// in, which the thread must hold; and the seq the window ends before.
func read(ctx context.Context, db *store.DB, in Input) ([]org.Message, Round, int64, error) {
	convID, through, limit := in.Prompt.ConversationID, in.Conversation.SummaryThroughSeq, window(in.Tier)
	before, bound := in.Prompt.Seq, in.Prompt.Seq
	if in.Parked != "" {
		before, bound = 0, 0
	}
	var msgs []org.Message
	var round Round
	for {
		got, more, err := db.Messages(ctx, convID, before, page)
		if err != nil {
			return nil, Round{}, 0, err
		}
		msgs = append(got, msgs...)
		if in.Parked != "" && bound == 0 {
			if r, ok := Find(msgs, in.Parked); ok {
				round, bound = r, r.Start
			}
		}
		done := !more || len(got) == 0 || got[0].Seq <= through+1
		if done || (bound != 0 && textAfter(msgs, through, bound) > limit) {
			break
		}
		before = got[0].Seq
	}
	if in.Parked != "" && bound == 0 {
		return nil, Round{}, 0, errors.New("the round this run continues is no longer in the conversation")
	}
	return msgs, round, bound, nil
}

// textAfter is the bytes of text in msgs after through and before bound:
// what the window chooses from.
func textAfter(msgs []org.Message, through, bound int64) int {
	n := 0
	for _, m := range msgs {
		if inWindow(m, through, bound) {
			n += len(m.Content)
		}
	}
	return n
}

// inWindow reports whether m is text the window may hold: after the summary
// and before bound, or anywhere after the summary when bound is zero. Tool
// rows are left out.
func inWindow(m org.Message, through, bound int64) bool {
	return m.Kind == org.MessageText && m.Content != "" && m.Seq > through && (bound == 0 || m.Seq < bound)
}

// windowOf is the newest text of msgs that fits limit, oldest first, with
// messages in a row from one side joined, since some providers refuse two
// turns in a row from one side; the bytes it holds; whether it left out text
// the summary does not cover; and the seq it starts at, bound when it holds
// nothing.
func windowOf(msgs []org.Message, through, bound int64, limit int) (hist []agent.Message, used int, behind bool, from int64) {
	from = bound
	for _, m := range slices.Backward(msgs) {
		if !inWindow(m, through, bound) {
			continue
		}
		if used+len(m.Content) > limit {
			behind = true
			break
		}
		used += len(m.Content)
		from = m.Seq
		// Owner- and bot-written text: the conversation's own messages, each
		// in the role of who wrote it.
		hist = prepend(hist, m)
	}
	slices.Reverse(hist)
	return hist, used, behind, from
}

// prepend adds m to newest, the history read back from its end, joining it
// to the message it comes before when both are from the same side.
func prepend(newest []agent.Message, m org.Message) []agent.Message {
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

// withSummary puts the conversation's summary first in hist, as the owner's,
// joined to the first message when that is the owner's too, and returns the
// bytes it took.
func withSummary(hist []agent.Message, summary string) ([]agent.Message, int) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return hist, 0
	}
	// Model-written text from the conversation's own messages: the summary,
	// under its lead line, in the history and never in the system text.
	text := summaryLead + cut(summary, summaryCap-len(summaryLead))
	if len(hist) > 0 && hist[0].Role == agent.RoleOwner {
		hist[0].Text = text + "\n\n" + hist[0].Text
		return hist, len(text)
	}
	return append([]agent.Message{{Role: agent.RoleOwner, Text: text}}, hist...), len(text)
}

// handbookBlock is the "## Handbook" block for pages, cut at its cap with a
// notice should the headings take it over. The cut is at the end of a line
// and closes a code fence left open, so the notice and the notes after the
// block are never read as part of a page's code.
func handbookBlock(pages []memory.Page) string {
	// Owner-supplied text, which may be pasted from a web page: the pages'
	// titles and Markdown, under the "## Handbook" heading.
	b := handbook.Render(pages)
	const notice = "\n(The handbook was cut here: it is over its size.)\n"
	if len(b) <= handbookCap {
		return b
	}
	b = cut(b, handbookCap-len(notice)-len("```\n"))
	b = b[:strings.LastIndex(b, "\n")+1]
	if fence := openFence(b); fence != "" {
		b += fence + "\n"
	}
	return b + notice
}

// openFence is the fence ("```" or "~~~") of a code block md ends inside,
// or "" when it ends outside one.
func openFence(md string) string {
	open := ""
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimLeft(line, " ")
		if len(line)-len(t) > 3 {
			continue
		}
		for _, f := range []string{"```", "~~~"} {
			switch {
			case open == "" && strings.HasPrefix(t, f):
				open = f
			case open == f && strings.HasPrefix(t, f):
				open = ""
			}
		}
	}
	return open
}

// notesBlock is the "## Your notes" block: pinned notes first, then the
// newest, one line each with its date, as many as fit the cap and then how
// many older ones were left out.
func notesBlock(notes []memory.Note) string {
	if len(notes) == 0 {
		return ""
	}
	ordered := slices.Clone(notes)
	slices.SortStableFunc(ordered, func(a, b memory.Note) int {
		if a.Pinned != b.Pinned {
			if a.Pinned {
				return -1
			}
			return 1
		}
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	var b strings.Builder
	b.WriteString("## Your notes\n\n")
	const room = 32 // for the line that counts the notes left out
	for i, n := range ordered {
		// Bot-written text, which the owner may edit: each note on one line
		// under "## Your notes".
		line := "- " + n.CreatedAt.Format("2006-01-02") + ": " + memory.OneLine(n.Text) + "\n"
		if b.Len()+len(line) > notesCap-room {
			fmt.Fprintf(&b, "(and %d older notes)\n", len(ordered)-i)
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// attachLines is a "File attached:" line for each file in refs, as many as
// fit the cap and then how many more there are.
func attachLines(refs []org.FileRef) string {
	var b strings.Builder
	const room = 32 // for the line that counts the files left out
	for i, f := range refs {
		// Owner-supplied text: the dropped file's name and where it is, on
		// one line.
		line := fmt.Sprintf("File attached: %s (%s) at %s\n", memory.OneLine(f.Name), size(f.Path), memory.OneLine(f.Path))
		if b.Len()+len(line) > attachCap-room {
			fmt.Fprintf(&b, "(and %d more files)\n", len(refs)-i)
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// size is the size of the file at path in words, or that it is gone.
func size(path string) string {
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return "no longer there"
	case info.Size() < 1<<10:
		return fmt.Sprintf("%d bytes", info.Size())
	case info.Size() < 1<<20:
		return fmt.Sprintf("%d KB", (info.Size()+1<<10-1)>>10)
	}
	return fmt.Sprintf("%.1f MB", float64(info.Size())/(1<<20))
}

// cut is s cut to at most n bytes on a character boundary.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	n = max(n, 0)
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// joinBlocks joins the non-empty blocks with a blank line between them.
func joinBlocks(blocks ...string) string {
	var kept []string
	for _, b := range blocks {
		if b = strings.TrimRight(b, "\n"); b != "" {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "\n\n")
}
