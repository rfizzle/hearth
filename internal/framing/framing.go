package framing

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/rfizzle/hearth/internal/org/memory"
)

//go:embed consolidate.md
var consolidateText string

//go:embed dm.md
var dmText string

//go:embed draft.md
var draftText string

// render fills one of the embedded texts; the text and the data are both
// ours, so a failure is a bug.
func render(name, text string, data any) string {
	t := template.Must(template.New(name).Option("missingkey=error").Parse(text))
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		panic("framing: " + err.Error())
	}
	return b.String()
}

// DraftInput is what the drafting text is about.
type DraftInput struct {
	Role     string // the template's role, when the owner started from one
	Starting string // the template's job description, inserted verbatim; empty for a blank start
}

// RenderDraft returns the system text for turning an owner's description
// into a proposed name, role, job description and working style.
func RenderDraft(in DraftInput) string { return render("draft", draftText, in) }

// Input is what the framing is about.
type Input struct {
	OrgName        string
	BotName        string
	Role           string
	JobDescription string // inserted verbatim
	FolderPath     string // where the bot's deliverables go
}

// Render returns the system text for a direct conversation with the owner.
func Render(in Input) string { return render("dm", dmText, in) }

// consolidateRoom is how many notes under the cap the tidied set leaves free,
// so the bot has room to write more before it is tidied again.
const consolidateRoom = 20

// ConsolidateMax is the most notes the tidied unpinned set may have when
// pinned notes are kept beside it: the cap less the pinned ones less some
// room, and at least one.
func ConsolidateMax(pinned int) int { return max(memory.Cap-pinned-consolidateRoom, 1) }

type consolidateData struct {
	Pinned, Unpinned []string
	Max              int
}

// RenderConsolidate returns the system text that asks for the unpinned
// notes to be merged into fewer, at most ConsolidateMax(len(pinned)). Pinned
// notes are shown as fixed. Notes are inserted verbatim, with their dates.
func RenderConsolidate(pinned, unpinned []memory.Note) string {
	return render("consolidate", consolidateText, consolidateData{
		Pinned: noteLines(pinned), Unpinned: noteLines(unpinned), Max: ConsolidateMax(len(pinned)),
	})
}

func noteLines(notes []memory.Note) []string {
	out := make([]string, len(notes))
	for i, n := range notes {
		out[i] = n.CreatedAt.Format("2006-01-02") + ": " + n.Text
	}
	return out
}
