package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/tools"
)

// fake is a bot's notes in memory.
type fake struct {
	notes []memory.Note
	fail  error
}

func (f *fake) store() Store {
	return Store{
		BotID: "bot-1", ConversationID: "conv-1", MessageID: "msg-1",
		Add: func(_ context.Context, n memory.Note) error {
			if f.fail != nil {
				return f.fail
			}
			f.notes = append(f.notes, n)
			return nil
		},
		Count: func(context.Context, string) (int, error) { return len(f.notes), nil },
	}
}

func call(t *testing.T, d tools.Definition, args string) (tools.Result, error) {
	t.Helper()
	return d.Execute(t.Context(), tools.Call{Args: []byte(args)})
}

func TestRemember_StoresTheNoteWithItsLink(t *testing.T) {
	f := &fake{}
	d := Definition(f.store())
	if d.Name != "remember" || d.Capability != org.Remember || d.Tier != tools.TierRead || !d.Standing() {
		t.Fatalf("definition = %+v", d)
	}
	res, err := call(t, d, `{"text":"  prefers short answers "}`)
	if err != nil || res.Text != "Noted." {
		t.Fatalf("call = %q, %v, want Noted.", res.Text, err)
	}
	want := memory.Note{BotID: "bot-1", Text: "prefers short answers", ConversationID: "conv-1", MessageID: "msg-1"}
	if len(f.notes) != 1 || f.notes[0] != want {
		t.Fatalf("notes = %+v, want %+v", f.notes, want)
	}
	if got := d.Describe([]byte(`{"text":" prefers short answers "}`)); got != "Remember: prefers short answers" {
		t.Errorf("describe = %q", got)
	}
	if got := d.Describe([]byte(`not json`)); got != "Remember: " {
		t.Errorf("describe of garbage = %q", got)
	}
	for _, w := range []string{"preferences", "decisions", "Never write down progress"} {
		if !strings.Contains(d.Description, w) {
			t.Errorf("description is missing %q", w)
		}
	}
}

func TestRemember_AtTheCapTheReplyNamesTheTidy(t *testing.T) {
	f := &fake{notes: make([]memory.Note, memory.Cap-2)}
	d := Definition(f.store())
	if res, err := call(t, d, `{"text":"one"}`); err != nil || res.Text != "Noted." {
		t.Fatalf("below the cap: %q, %v", res.Text, err)
	}
	res, err := call(t, d, `{"text":"two"}`)
	if err != nil || res.Text != Full || !strings.Contains(res.Text, "tidied") {
		t.Fatalf("at the cap: %q, %v, want %q", res.Text, err, Full)
	}
	if len(f.notes) != memory.Cap {
		t.Errorf("the note at the cap was not kept: %d notes", len(f.notes))
	}
}

func TestRemember_RefusesWhatIsNotANote(t *testing.T) {
	f := &fake{}
	d := Definition(f.store())
	for _, args := range []string{`{"text":""}`, `{"text":"   "}`, `{}`, `[]`, `{"text":"` + strings.Repeat("x", maxText+1) + `"}`} {
		if d.Validate([]byte(args)) == nil {
			t.Errorf("Validate(%.40s) accepted it", args)
		}
		if _, err := call(t, d, args); err == nil {
			t.Errorf("a call with %.40s was run", args)
		}
	}
	if len(f.notes) != 0 {
		t.Errorf("notes = %+v, want none", f.notes)
	}
}

func TestRemember_ReportsAStoreFailure(t *testing.T) {
	f := &fake{fail: errors.New("disk full")}
	if _, err := call(t, Definition(f.store()), `{"text":"x"}`); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want the store's failure", err)
	}
}
