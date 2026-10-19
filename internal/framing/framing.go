package framing

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed dm.md
var dmText string

// Input is what the framing is about.
type Input struct {
	OrgName        string
	BotName        string
	Role           string
	JobDescription string // inserted verbatim
	FolderPath     string // where the bot's deliverables go
}

// Render returns the system text for a direct conversation with the owner.
func Render(in Input) string {
	dm := template.Must(template.New("dm").Option("missingkey=error").Parse(dmText))
	var b strings.Builder
	if err := dm.Execute(&b, in); err != nil {
		panic("framing: " + err.Error()) // the template and Input are both ours
	}
	return b.String()
}
