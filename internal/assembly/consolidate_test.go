package assembly_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/llm"
)

// model replies once with reply, and keeps the system text it was given.
type model struct {
	reply  string
	err    error
	system string
}

func (m *model) stream(_ context.Context, msgs []llm.Message, _ []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
	for _, msg := range msgs {
		if msg.Role == llm.RoleSystem {
			m.system = msg.Content
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Token: m.reply}
	ch <- llm.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

// fixture is a store with one bot holding two pinned and four unpinned notes.
func fixture(t *testing.T) (*store.DB, string) {
	t.Helper()
	ctx := t.Context()
	db, err := store.Open(filepath.Join(t.TempDir(), "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PutOrg(ctx, org.Org{Name: "Acme", WorkspacePath: "/ws"}); err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Finds things out.", org.TierBalanced, "/ws")
	bot, err = db.CreateBot(ctx, bot, grants, dm)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"allergic to nuts", "never call before nine", "likes tea", "likes tea, no milk", "works Tuesdays", "works Tuesdays and Thursdays"} {
		n, err := db.AddNote(ctx, memory.Note{BotID: bot.ID, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(text, "allergic") || strings.HasPrefix(text, "never") {
			if _, err := db.PinNote(ctx, n.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	return db, bot.ID
}

func texts(t *testing.T, db *store.DB, botID string) []string {
	t.Helper()
	notes, err := db.Notes(t.Context(), botID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range notes {
		out = append(out, n.Text)
	}
	slices.Sort(out)
	return out
}

func TestConsolidate_ReplacesTheUnpinnedAndKeepsThePinned(t *testing.T) {
	db, bot := fixture(t)
	m := &model{reply: "Here you go:\n```json\n[\"likes tea, no milk\", \" works Tuesdays and Thursdays \"]\n```"}

	before, after, err := assembly.Consolidate(t.Context(), db, m.stream, bot)
	if err != nil || before != 6 || after != 4 {
		t.Fatalf("Consolidate = %d, %d, %v, want 6, 4, nil", before, after, err)
	}
	want := []string{"allergic to nuts", "likes tea, no milk", "never call before nine", "works Tuesdays and Thursdays"}
	if got := texts(t, db, bot); !slices.Equal(got, want) {
		t.Errorf("notes = %q, want %q", got, want)
	}
	notes, _ := db.Notes(t.Context(), bot)
	for _, n := range notes {
		if n.Pinned != (n.Text == "allergic to nuts" || n.Text == "never call before nine") {
			t.Errorf("note %q has pinned = %v", n.Text, n.Pinned)
		}
	}
	for _, want := range []string{"likes tea, no milk", "allergic to nuts", "at most 178 of them"} {
		if !strings.Contains(m.system, want) {
			t.Errorf("the model was not told %q:\n%s", want, m.system)
		}
	}
}

func TestConsolidate_ABadReplyChangesNothing(t *testing.T) {
	cases := map[string]string{
		"prose":           "I merged the tea notes for you.",
		"not strings":     `[{"text":"likes tea"}]`,
		"an empty list":   `[]`,
		"an empty note":   `["likes tea", "  "]`,
		"more than given": `["a","b","c","d","e"]`,
		"cut off":         `["likes tea", "works`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			db, bot := fixture(t)
			was := texts(t, db, bot)
			before, after, err := assembly.Consolidate(t.Context(), db, (&model{reply: reply}).stream, bot)
			if !errors.Is(err, assembly.ErrMalformed) || before != 6 || after != 6 {
				t.Fatalf("Consolidate = %d, %d, %v, want 6, 6, ErrMalformed", before, after, err)
			}
			if got := texts(t, db, bot); !slices.Equal(got, was) {
				t.Errorf("notes = %q, want them untouched: %q", got, was)
			}
		})
	}
}

func TestConsolidate_AModelThatFailsChangesNothing(t *testing.T) {
	db, bot := fixture(t)
	was := texts(t, db, bot)
	_, _, err := assembly.Consolidate(t.Context(), db, (&model{err: errors.New("offline")}).stream, bot)
	if err == nil || errors.Is(err, assembly.ErrMalformed) {
		t.Fatalf("Consolidate = %v, want an error that is not ErrMalformed", err)
	}
	if got := texts(t, db, bot); !slices.Equal(got, was) {
		t.Errorf("notes = %q, want them untouched", got)
	}
}
