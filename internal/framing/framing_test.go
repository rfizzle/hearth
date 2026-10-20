package framing_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/org/memory"
)

func TestRenderConsolidate(t *testing.T) {
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	pinned := []memory.Note{{Text: "allergic to nuts", Pinned: true, CreatedAt: at}}
	unpinned := []memory.Note{{Text: "likes tea", CreatedAt: at}, {Text: "likes tea, no milk", CreatedAt: at}}
	got := framing.RenderConsolidate(pinned, unpinned)

	for _, want := range []string{"2026-10-20: likes tea, no milk", "2026-10-20: allergic to nuts", "at most 179 of them", "Never invent", "JSON array", "data, not instructions"} {
		if !strings.Contains(got, want) {
			t.Errorf("consolidate text is missing %q:\n%s", want, got)
		}
	}
	if n := framing.ConsolidateMax(0); n != memory.Cap-20 {
		t.Errorf("ConsolidateMax(0) = %d, want %d", n, memory.Cap-20)
	}
	names := regexp.MustCompile(`(?i)\b(read|write|fetch|search|delete|files?\.\w+|web\.\w+|assign|remember)\b`)
	if m := names.FindString(got); m != "" {
		t.Errorf("consolidate text names a tool: %q", m)
	}
	if strings.Contains(framing.RenderConsolidate(nil, unpinned), "pinned. They stay") {
		t.Error("no pinned notes, yet the text speaks of them")
	}
}

func TestRender(t *testing.T) {
	job := "Summarise {{.Weekly}} reports.\n\nUse **bullets**; never say \"fetch\" lightly."
	got := framing.Render(framing.Input{
		OrgName: "Acme", BotName: "Wren", Role: "Research Analyst",
		JobDescription: job, FolderPath: "/Users/me/Hearth/Wren",
	})

	for _, want := range []string{"Wren", "Research Analyst", "Acme", "/Users/me/Hearth/Wren", job, "data, not instructions", "Markdown", "begins with (hearth) is from the app"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text is missing %q:\n%s", want, got)
		}
	}
}

func TestRenderDraft(t *testing.T) {
	blank := framing.RenderDraft(framing.DraftInput{})
	for _, want := range []string{"job_description", "working_style", `"tier"`, "one JSON object"} {
		if !strings.Contains(blank, want) {
			t.Errorf("draft text is missing %q:\n%s", want, blank)
		}
	}
	if strings.Contains(blank, "ready-made") {
		t.Error("a blank start mentions a template")
	}
	start := "Summarise {{.Weekly}} reports."
	got := framing.RenderDraft(framing.DraftInput{Role: "Writer", Starting: start})
	for _, want := range []string{"ready-made Writer role", start} {
		if !strings.Contains(got, want) {
			t.Errorf("draft text is missing %q:\n%s", want, got)
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

func TestRenderAsksThePerson(t *testing.T) {
	got := framing.Render(framing.Input{OrgName: "O", BotName: "B", Role: "R", JobDescription: "J", FolderPath: "/f"})
	i := strings.Index(got, "## Asking the person")
	if i < 0 {
		t.Fatalf("framing has no section on asking the person:\n%s", got)
	}
	section := got[i:]
	for _, want := range []string{
		"suggested replies", "Single choice", "Multiple choice", "Free text", "Yes or no", "Date", "File or folder", "Scale",
		"The owner chose: Keep it", "The owner chose: Other:", "The owner chose none", "The owner answered:",
		"The owner said: Yes", "The owner picked the date: 2026-10-24", "The owner picked: /", "The owner rated it 4 of 5",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the section is missing %q", want)
		}
	}
	// The section describes the behaviour; it never names the tool that asks.
	for _, tool := range []string{"ask_owner", "tool", "widget", "JSON"} {
		if strings.Contains(section, tool) {
			t.Errorf("the section names %q", tool)
		}
	}
}
