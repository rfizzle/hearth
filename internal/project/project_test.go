package project

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

// The values below are written to disk; renumbering one changes what every
// stored row means.
func TestEnumValuesAreStable(t *testing.T) {
	got := []int{
		int(Proposed), int(Active), int(Done), int(Archived),
		int(LightNone), int(LightGreen), int(LightYellow), int(LightRed),
		int(org.ConversationProject),
	}
	want := []int{1, 2, 3, 4, 1, 2, 3, 4, 4}
	if len(got) != len(want) {
		t.Fatalf("enum values = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("enum values = %v, want %v", got, want)
		}
	}
	if CanPropose != "project.propose" {
		t.Fatalf("CanPropose = %q", CanPropose)
	}
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		name    string
		c       Counts
		light   Light
		label   string
		allDone bool
	}{
		{"no tasks", Counts{}, LightNone, "No tasks yet", false},
		{"working", Counts{Total: 3, Done: 1, Working: 2}, LightGreen, "On track", false},
		{"all done", Counts{Total: 2, Done: 2}, LightGreen, "All tasks done", true},
		{"cancelled count as finished", Counts{Total: 3, Done: 1, Cancelled: 2}, LightGreen, "All tasks done", true},
		{"only cancelled", Counts{Total: 1, Cancelled: 1}, LightGreen, "All tasks done", true},
		{"cancelled but one working", Counts{Total: 3, Done: 1, Cancelled: 1, Working: 1}, LightGreen, "On track", false},
		{"waiting on the owner", Counts{Total: 2, Done: 1, WaitingOwner: 1}, LightYellow, "Waiting on you", false},
		{"failed", Counts{Total: 2, Done: 1, Failed: 1}, LightYellow, "Waiting on you", false},
		{"paused on budget", Counts{Total: 2, Done: 1, PausedBudget: 1}, LightRed, "Paused on budget", false},
		{"red over yellow", Counts{Total: 3, PausedBudget: 1, WaitingOwner: 1, Failed: 1}, LightRed, "Paused on budget", false},
		{"yellow over all finished", Counts{Total: 3, Done: 1, Cancelled: 1, Failed: 1}, LightYellow, "Waiting on you", false},
		{"red over all else", Counts{Total: 2, Done: 1, PausedBudget: 1}, LightRed, "Paused on budget", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := StatusOf(c.c)
			if s.Light != c.light || s.Label != c.label || s.AllDone != c.allDone || s.Counts != c.c {
				t.Fatalf("StatusOf(%+v) = %+v, want %v %q all done %v", c.c, s, c.light, c.label, c.allDone)
			}
		})
	}
}

var goodBrief = Brief{Goal: "Launch", Context: "We sell mugs", DoneLooksLike: "A live page", DoneWhen: "Orders arrive"}

func TestNew(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "Hearth")
	p, conv, err := New("  Launch the shop ", goodBrief, "2026-12-31", root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Launch the shop" || p.State != Active || p.TargetDate != "2026-12-31" || p.Brief != goodBrief {
		t.Fatalf("project = %+v", p)
	}
	if want := filepath.Join(root, "Projects", "Launch the shop"); p.FolderPath != want {
		t.Fatalf("FolderPath = %q, want %q", p.FolderPath, want)
	}
	if conv.Kind != org.ConversationProject || conv.Title != "Launch the shop" || conv.BotID != "" {
		t.Fatalf("conversation = %+v", conv)
	}
}

func TestNew_TheFolderIsOneElementUnderProjects(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "Hearth")
	p, _, err := New("../x/y", goodBrief, "", root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p.FolderPath) != filepath.Join(root, "Projects") {
		t.Fatalf("FolderPath = %q, want one element under Projects", p.FolderPath)
	}
	// The Inbox rule is a bot's; a project may be called Inbox.
	if _, _, err := New("Inbox", goodBrief, "", root); err != nil {
		t.Fatalf("New(Inbox) = %v", err)
	}
}

func TestValidate_Refuses(t *testing.T) {
	with := func(f func(*Brief)) Brief {
		b := goodBrief
		f(&b)
		return b
	}
	long := strings.Repeat("é", 4001)
	cases := []struct {
		name  string
		pname string
		brief Brief
		date  string
	}{
		{"empty name", " ", goodBrief, ""},
		{"long name", strings.Repeat("é", 81), goodBrief, ""},
		{"empty goal", "P", with(func(b *Brief) { b.Goal = " " }), ""},
		{"empty context", "P", with(func(b *Brief) { b.Context = "" }), ""},
		{"empty done looks like", "P", with(func(b *Brief) { b.DoneLooksLike = "" }), ""},
		{"empty done when", "P", with(func(b *Brief) { b.DoneWhen = "" }), ""},
		{"long goal", "P", with(func(b *Brief) { b.Goal = long }), ""},
		{"long done when", "P", with(func(b *Brief) { b.DoneWhen = long }), ""},
		{"date words", "P", goodBrief, "tomorrow"},
		{"date slashes", "P", goodBrief, "2026/12/31"},
		{"date not a day", "P", goodBrief, "2026-02-30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := Validate(c.pname, c.brief, c.date); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate = %v, want ErrInvalid", err)
			}
			if _, _, err := New(c.pname, c.brief, c.date, "/Hearth"); !errors.Is(err, ErrInvalid) {
				t.Fatalf("New = %v, want ErrInvalid", err)
			}
			if _, err := Propose(c.pname, c.brief, "b", "c"); c.date == "" && !errors.Is(err, ErrInvalid) {
				t.Fatalf("Propose = %v, want ErrInvalid", err)
			}
		})
	}
	if err := Validate(strings.Repeat("é", 80), with(func(b *Brief) { b.Goal = strings.Repeat("é", 4000) }), "2026-02-28"); err != nil {
		t.Fatalf("the limits themselves are allowed: %v", err)
	}
}

func TestProposeThenActivate_KeepsTheIDAndTheProposer(t *testing.T) {
	p, err := Propose("Launch", goodBrief, "bot1", "conv1")
	if err != nil {
		t.Fatal(err)
	}
	if p.State != Proposed || p.FolderPath != "" || p.ConversationID != "" || p.ProposedByBotID != "bot1" || p.ProposedInConversationID != "conv1" {
		t.Fatalf("proposal = %+v", p)
	}
	p.ID = "p1"
	edited := goodBrief
	edited.Goal = "Launch sooner"
	a, conv, err := Activate(p, "Launch two", edited, "2027-01-01", "/Hearth")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "p1" || a.ProposedByBotID != "bot1" || a.ProposedInConversationID != "conv1" ||
		a.State != Active || a.Name != "Launch two" || a.Brief != edited || a.TargetDate != "2027-01-01" ||
		a.FolderPath != filepath.Join("/Hearth", "Projects", "Launch two") {
		t.Fatalf("activated = %+v", a)
	}
	if conv.Kind != org.ConversationProject || conv.ProjectID != "p1" {
		t.Fatalf("conversation = %+v", conv)
	}
	if _, _, err := Activate(p, "", edited, "", "/Hearth"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Activate with no name = %v", err)
	}
}
