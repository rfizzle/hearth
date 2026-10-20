package framing

import (
	_ "embed"
	"strings"
	"text/template"
)

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
