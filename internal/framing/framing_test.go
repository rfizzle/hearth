package framing_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/framing"
)

func TestRender(t *testing.T) {
	job := "Summarise {{.Weekly}} reports.\n\nUse **bullets**; never say \"fetch\" lightly."
	got := framing.Render(framing.Input{
		OrgName: "Acme", BotName: "Wren", Role: "Research Analyst",
		JobDescription: job, FolderPath: "/Users/me/Hearth/Wren",
	})

	for _, want := range []string{"Wren", "Research Analyst", "Acme", "/Users/me/Hearth/Wren", job, "data, not instructions", "Markdown"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text is missing %q:\n%s", want, got)
		}
	}
}

func TestRenderNamesNoTool(t *testing.T) {
	// The job description is the owner's; only the fixed text is checked.
	got := framing.Render(framing.Input{OrgName: "O", BotName: "B", Role: "R", JobDescription: "J", FolderPath: "/f"})
	names := regexp.MustCompile(`(?i)\b(read|write|fetch|search|delete|files?\.\w+|web\.\w+|assign|remember)\b`)
	if m := names.FindString(got); m != "" {
		t.Errorf("framing names a tool: %q", m)
	}
}
