package park

import (
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
)

func TestResume_History(t *testing.T) {
	owner := func(s string) org.Message { return org.Message{Kind: org.MessageText, Content: s} }
	bot := func(run, s string) org.Message {
		return org.Message{AuthorBotID: "b", RunID: run, Kind: org.MessageText, Content: s}
	}

	t.Run("a fresh run reads the text, joined by side", func(t *testing.T) {
		msgs := thread(owner("hi"), owner("are you there?"), bot("r0", "Yes."), call("r0", "c1"), result("r0", "c1", "ok"), bot("r0", ""))
		got, prompt, err := Resume{}.History(msgs, "", "go on")
		if err != nil || prompt != "go on" {
			t.Fatalf("History = %q, %v", prompt, err)
		}
		if len(got) != 2 || got[0].Role != agent.RoleOwner || got[0].Text != "hi\n\nare you there?" ||
			got[1].Role != agent.RoleBot || got[1].Text != "Yes." {
			t.Fatalf("history = %+v, want the owner's two messages joined, then the reply", got)
		}
	})

	t.Run("older text falls off the budget", func(t *testing.T) {
		big := strings.Repeat("x", historyBudget)
		got, _, _ := Resume{}.History(thread(owner(big), bot("r0", "short")), "", "p")
		if len(got) != 1 || got[0].Text != "short" {
			t.Fatalf("history = %d messages, want only the newest that fit", len(got))
		}
	})

	t.Run("a resumed run reads the text before its round, then the round", func(t *testing.T) {
		msgs := thread(owner("tidy"), bot("r1", "On it."), call("r1", "c1"), requested("r1"))
		got, prompt, err := Resume{}.History(msgs, "r1", Note)
		if err != nil || prompt != Note {
			t.Fatalf("History = %q, %v", prompt, err)
		}
		if len(got) != 3 || got[0].Role != agent.RoleOwner || got[1].Text != "On it." || got[2].CallID != "c1" || got[2].Text != NotRun {
			t.Fatalf("history = %+v", got)
		}
	})

	t.Run("a resumed run whose round is gone fails", func(t *testing.T) {
		if _, _, err := (Resume{}).History(thread(owner("tidy")), "r1", Note); err == nil {
			t.Fatal("History found a round that is not there")
		}
	})
}
