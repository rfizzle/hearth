package assembly_test

import (
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/intro"
)

// A bot's introduction is the app's card: no part of it reaches the system
// text, the history or the prompt of a run.
// See docs/model.md#people
func TestBuild_NeverCarriesTheIntroduction(t *testing.T) {
	w := newWorld(t)
	w.talk(t, 6, 80)
	prompt := w.say(t, false, "And now?")
	bot := w.dana
	bot.Introduction = intro.Introduction{
		Purpose: "ZEBRA-PURPOSE I keep the office.",
		How: []intro.Line{
			{Symbol: "folder", Text: "ZEBRA-HOW-ONE"},
			{Symbol: "pencil.line", Text: "ZEBRA-HOW-TWO"},
			{Symbol: "doc.text", Text: "ZEBRA-HOW-THREE"},
		},
		Try: []string{"ZEBRA-TRY-ONE", "ZEBRA-TRY-TWO", "ZEBRA-TRY-THREE"},
	}

	out, err := assembly.Build(t.Context(), w.db, assembly.Input{
		Org: w.org, Bot: bot, Conversation: w.conversation(t), Prompt: prompt, Tier: org.TierBalanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	all := out.System + "\n" + out.Prompt
	for _, h := range out.History {
		all += "\n" + h.Text
	}
	if strings.Contains(all, "ZEBRA") {
		t.Errorf("the introduction reached the run's context:\n%s", all)
	}
}
