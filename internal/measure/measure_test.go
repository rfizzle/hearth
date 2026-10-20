package measure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

var pinned = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

type fakes struct {
	bots     []org.Bot
	projects []project.Project
	counts   map[string]project.Counts
	months   map[string]Month
	inbox    int
	notes    map[string]int
	failBots bool
}

func (f *fakes) Bots(context.Context) ([]org.Bot, error) {
	if f.failBots {
		return nil, errors.New("disk")
	}
	return f.bots, nil
}
func (f *fakes) Projects(context.Context, bool) ([]project.Project, error) { return f.projects, nil }
func (f *fakes) Month(_ context.Context, m string) (Month, error)          { return f.months[m], nil }
func (f *fakes) Open(context.Context) (int, error)                         { return f.inbox, nil }
func (f *fakes) Count(_ context.Context, id string) (int, error)           { return f.notes[id], nil }
func (f *fakes) Used(_ context.Context, id string) (int64, int64, error) {
	if id == "" {
		return 100, 1000, nil
	}
	return 250, 1000, nil
}

func rig(t *testing.T) (Deps, *fakes) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &fakes{
		bots:     []org.Bot{{ID: "bot1", Name: "Ada"}, {ID: "bot2", Name: "Bo"}},
		projects: []project.Project{{ID: "p1", Name: "Launch", FolderPath: dir}, {ID: "p2", Name: "Empty"}},
		counts:   map[string]project.Counts{"p1": {Total: 7, Done: 3, Working: 4}},
		months: map[string]Month{"2026-10": {
			Org:  Spend{CostMicros: 3_500_000, Tokens: 900, BudgetCents: 2000, Priced: true},
			Bots: map[string]Spend{"bot1": {CostMicros: 1_000_000, Tokens: 400, Priced: true}, "bot2": {Tokens: 5}},
		}, "2025-01": {Org: Spend{Priced: true}}},
		inbox: 2,
		// 2025-01 has no usage, which is priced: nothing was unpriced.
		notes: map[string]int{"bot1": 3},
	}
	return Deps{
		Roster: f, Projects: f, Ledger: f, Inbox: f, Notes: f, Handbook: f,
		Status: func(_ context.Context, id string) (project.Status, error) {
			return project.StatusOf(f.counts[id]), nil
		},
		Now: func() time.Time { return pinned },
	}, f
}

func TestParseID(t *testing.T) {
	tests := []struct {
		in   string
		want ID
		bad  string
	}{
		{in: "project.tasks/p1", want: ID{"project.tasks", "p1", ""}},
		{in: "usage.cost/org/2026-10", want: ID{"usage.cost", "org", "2026-10"}},
		{in: "usage.cost/me/current", want: ID{"usage.cost", "me", "current"}},
		{in: "inbox.open", want: ID{Family: "inbox.open"}},
		{in: "", bad: "family/subject"},
		{in: "cats.count/x", bad: "not a measure"},
		{in: "inbox.open/x", bad: "inbox.open is written"},
		{in: "project.tasks", bad: "written project.tasks/<project id>"},
		{in: "project.tasks/p1/2026-10", bad: "takes no month"},
		{in: "usage.cost/org", bad: "needs a month"},
		{in: "usage.cost/org/2026-13", bad: "yyyy-mm"},
		{in: "usage.cost/org/2026-10/x", bad: "more parts"},
		{in: "project.tasks/../etc", bad: "letters, digits"},
	}
	for _, tt := range tests {
		got, err := ParseID(tt.in)
		if tt.bad != "" {
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.bad) {
				t.Errorf("ParseID(%q) = %v, want a refusal with %q", tt.in, err, tt.bad)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseID(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
		if got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestResolve(t *testing.T) {
	d, _ := rig(t)
	scope := Scope{BotID: "bot1", ProjectID: "p1"}
	tests := []struct {
		in    string
		scope Scope
		want  string
		bad   string
	}{
		{"project.tasks/this", scope, "project.tasks/p1", ""},
		{"usage.cost/me/current", scope, "usage.cost/bot1/2026-10", ""},
		{"usage.tokens/org/current", scope, "usage.tokens/org/2026-10", ""},
		{"handbook.meter/org", scope, "handbook.meter/everyone", ""},
		{"notes.count/me", scope, "notes.count/bot1", ""},
		{"project.tasks/p2", scope, "project.tasks/p2", ""},
		{"team.size", Scope{}, "team.size", ""},
		{"project.tasks/this", Scope{BotID: "bot1"}, "", "not in a project"},
		{"notes.count/me", Scope{ProjectID: "p1"}, "", "bot that is asking"},
	}
	for _, tt := range tests {
		id, err := ParseID(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		got, err := d.Resolve(id, tt.scope)
		if tt.bad != "" {
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.bad) {
				t.Errorf("Resolve(%q) = %v, want a refusal with %q", tt.in, err, tt.bad)
			}
			continue
		}
		if err != nil || got.String() != tt.want {
			t.Errorf("Resolve(%q) = %v, %v; want %s", tt.in, got, err, tt.want)
		}
	}
}

func read(t *testing.T, d Deps, in ...string) []Value {
	t.Helper()
	var ids []ID
	for _, s := range in {
		id, err := ParseID(s)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	vs, err := d.Read(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func TestRead_EachFamily(t *testing.T) {
	d, _ := rig(t)
	vs := read(t, d,
		"project.tasks/p1", "project.tasks/p2", "project.light/p1", "project.files/p1",
		"usage.cost/org/2026-10", "usage.tokens/bot1/2026-10", "usage.budget/org/2026-10",
		"inbox.open", "notes.count/bot1", "handbook.meter/bot1", "handbook.meter/everyone", "team.size")
	check := func(i int, f func(v Value) bool, what string) {
		t.Helper()
		if v := vs[i]; v.Missing != "" || !f(v) || !v.AsOf.Equal(pinned) {
			t.Errorf("%s: got %+v", what, v)
		}
	}
	check(0, func(v Value) bool { return v.Kind == Progress && v.Done == 3 && v.Total == 7 }, "3 of 7")
	check(1, func(v Value) bool { return v.Total == 0 && v.Word == "No tasks yet" }, "no tasks yet")
	check(2, func(v Value) bool { return v.Kind == Light && v.Tone == TonePositive && v.Word == "On track" }, "light")
	check(3, func(v Value) bool { return v.Number == 2 }, "files, one level, regular only")
	check(4, func(v Value) bool { return v.Number == 3_500_000 && v.Unit == "usd_micros" }, "org cost")
	check(5, func(v Value) bool { return v.Number == 400 }, "tokens")
	check(6, func(v Value) bool { return v.Done == 350 && v.Total == 2000 }, "budget cents")
	check(7, func(v Value) bool { return v.Number == 2 }, "inbox")
	check(8, func(v Value) bool { return v.Number == 3 }, "notes")
	check(9, func(v Value) bool { return v.Done == 250 && v.Total == 1000 }, "handbook")
	check(10, func(v Value) bool { return v.Done == 100 }, "handbook everyone")
	check(11, func(v Value) bool { return v.Number == 2 }, "team")
	if vs[0].Label != "Tasks done · Launch" {
		t.Errorf("label = %q", vs[0].Label)
	}
}

func TestRead_MissingIsAValueNotAnError(t *testing.T) {
	d, _ := rig(t)
	vs := read(t, d, "project.tasks/gone", "notes.count/gone", "usage.cost/bot2/2026-10", "usage.budget/bot1/2026-10", "usage.cost/nobody/2026-10", "usage.cost/org/2025-01")
	want := []string{NoSuchProject, NoSuchBot, "Not priced", "No budget set", "No such bot", ""}
	for i, v := range vs {
		if v.Missing != want[i] {
			t.Errorf("%s missing = %q, want %q", v.ID, v.Missing, want[i])
		}
	}
}

func TestID_Kind(t *testing.T) {
	for s, want := range map[string]Kind{"project.tasks/p1": Progress, "project.light/this": Light, "team.size": Number} {
		id, err := ParseID(s)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := id.Kind(); err != nil || got != want {
			t.Errorf("%s: kind %q, %v; want %q", s, got, err, want)
		}
	}
	if _, err := (ID{Family: "nope"}).Kind(); !errors.Is(err, ErrInvalid) {
		t.Errorf("an unknown family: %v", err)
	}
}

func TestRead_RefusesAnAliasAndReportsAFailedSource(t *testing.T) {
	d, f := rig(t)
	id, _ := ParseID("project.tasks/this")
	if _, err := d.Read(context.Background(), []ID{id}); !errors.Is(err, ErrInvalid) {
		t.Errorf("an alias read: %v", err)
	}
	f.failBots = true
	id, _ = ParseID("team.size")
	if _, err := d.Read(context.Background(), []ID{id}); err == nil || errors.Is(err, ErrInvalid) {
		t.Errorf("a failed source should be a plain error, got %v", err)
	}
}

func TestList_EveryIdThatResolvesNow(t *testing.T) {
	d, _ := rig(t)
	vs, err := d.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 2 projects x 3, (org + 2 bots) x 3 usage, inbox, 2 notes, (everyone + 2) handbook, team.
	if len(vs) != 6+9+1+2+3+1 {
		t.Fatalf("%d ids", len(vs))
	}
	if vs[0].ID != "project.tasks/p1" || vs[0].Label != "Tasks done · Launch" || !vs[0].AsOf.IsZero() {
		t.Errorf("first = %+v", vs[0])
	}
	for _, v := range vs {
		id, err := ParseID(v.ID)
		if err != nil {
			t.Errorf("%s does not parse: %v", v.ID, err)
		} else if r, err := d.Resolve(id, Scope{}); err != nil || r != id {
			t.Errorf("%s is not resolved: %v", v.ID, err)
		}
	}
}
