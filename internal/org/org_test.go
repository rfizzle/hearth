package org

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org/intro"
)

func TestFolders(t *testing.T) {
	grants := []Grant{
		{Capability: FilesRead, Mode: ModeAllowed, Folder: "/a"},
		{Capability: FilesRead, Mode: ModeAsk, Folder: "/b"},
		{Capability: FilesRead, Mode: ModeNotGranted, Folder: "/c"},
		{Capability: FilesWrite, Mode: ModeAllowed, Folder: "/d"},
		{Capability: FilesRead, Mode: ModeAllowed},
	}
	tests := []struct {
		atLeast Mode
		want    []string
	}{
		{ModeAsk, []string{"/a", "/b"}},
		{ModeAllowed, []string{"/a"}},
	}
	for _, tt := range tests {
		if got := Folders(grants, FilesRead, tt.atLeast); !slices.Equal(got, tt.want) {
			t.Errorf("Folders at least %d = %v, want %v", tt.atLeast, got, tt.want)
		}
	}
}

func TestNewBot_GrantsTheDefaultsAndNothingElse(t *testing.T) {
	root := filepath.Join("/", "Users", "me", "Hearth")
	bot, grants, dm := NewBot("Research Analyst", "Analyst", "Finds things out.", TierBalanced, root)

	folder := filepath.Join(root, "Research Analyst")
	want := Bot{
		Name:           "Research Analyst",
		Role:           "Analyst",
		JobDescription: "Finds things out.",
		Tier:           TierBalanced,
		Status:         BotIdle,
		FolderPath:     folder,
		Scope:          ScopeTeam,
		Introduction:   intro.Assemble("Analyst", "Finds things out."),
	}
	if !reflect.DeepEqual(bot, want) {
		t.Fatalf("bot = %+v, want %+v", bot, want)
	}
	if !strings.Contains(bot.Introduction.Purpose, "I'm your Analyst.") {
		t.Errorf("introduction = %+v, want one assembled from the role", bot.Introduction)
	}
	if dm != (Conversation{Kind: ConversationDM, Title: "Research Analyst"}) {
		t.Fatalf("dm = %+v", dm)
	}

	modes := []struct {
		capability Capability
		want       Mode
		folders    []string
	}{
		{FilesRead, ModeAllowed, []string{folder}},
		{FilesWrite, ModeAllowed, []string{folder}},
		{FilesDelete, ModeAsk, nil},
		{WebFetch, ModeAllowed, nil},
		{WebSearch, ModeAllowed, nil},
		{AskOwner, ModeAllowed, nil},
		{Remember, ModeAllowed, nil},
		{"calendar.read", ModeNotGranted, nil},
		{"shortcuts.run", ModeNotGranted, nil},
	}
	for _, m := range modes {
		if got := Allowed(grants, m.capability); got != m.want {
			t.Errorf("Allowed(%s) = %d, want %d", m.capability, got, m.want)
		}
		if got := FolderGrants(grants, m.capability); !slices.Equal(got, m.folders) {
			t.Errorf("FolderGrants(%s) = %q, want %q", m.capability, got, m.folders)
		}
	}
	if len(grants) != 7 {
		t.Fatalf("%d grants, want exactly the seven defaults: %+v", len(grants), grants)
	}
	for _, g := range grants {
		if g.BotID != "" {
			t.Fatalf("grant carries an id before the store saved it: %+v", g)
		}
	}
}

func TestAllowed_TheMostPermissiveGrantWins(t *testing.T) {
	grants := []Grant{
		{Capability: FilesWrite, Mode: ModeAsk, Folder: "/a"},
		{Capability: FilesWrite, Mode: ModeAllowed, Folder: "/b"},
		{Capability: FilesWrite, Mode: ModeNotGranted, Folder: "/c"},
	}
	if got := Allowed(grants, FilesWrite); got != ModeAllowed {
		t.Fatalf("Allowed = %d, want ModeAllowed", got)
	}
	if got := FolderGrants(grants, FilesWrite); !slices.Equal(got, []string{"/b"}) {
		t.Fatalf("FolderGrants = %q, want only the allowed folder", got)
	}
	if got := Allowed(nil, WebFetch); got != ModeNotGranted {
		t.Fatalf("no grants: Allowed = %d, want ModeNotGranted", got)
	}
}

func TestFolderName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Writer", "Writer"},
		{"Chief of Staff", "Chief of Staff"},
		{"  Planner  ", "Planner"},
		{"a/b", "a-b"},
		{`a\b:c`, "a-b-c"},
		{"..", "Bot"},
		{"../../etc", "-..-etc"},
		{".hidden", "hidden"},
		{"tab\there", "tab-here"},
		{"", "Bot"},
		{"   ", "Bot"},
	}
	for _, c := range cases {
		if got := FolderName(c.name); got != c.want {
			t.Errorf("FolderName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDefaultCaps(t *testing.T) {
	if got := DefaultCaps(); got != (Caps{Steps: 25, Wall: 10 * time.Minute}) {
		t.Fatalf("DefaultCaps = %+v", got)
	}
}

// The values below are written to disk; renumbering one changes what every
// stored row means.
func TestEnumValuesAreStable(t *testing.T) {
	got := []int{
		int(TierFast), int(TierBalanced), int(TierBest),
		int(BotIdle), int(BotWorking),
		int(ConversationDM), int(ConversationProject),
		int(MessageText), int(MessageToolCall), int(MessageToolResult), int(MessageSystem),
		int(RunQueued), int(RunWorking), int(RunDone), int(RunFailed), int(RunCancelled),
		int(ModeNotGranted), int(ModeAsk), int(ModeAllowed),
	}
	want := []int{1, 2, 3, 1, 2, 1, 4, 1, 2, 3, 9, 1, 2, 4, 5, 6, 0, 1, 2}
	if !slices.Equal(got, want) {
		t.Fatalf("enum values = %v, want %v", got, want)
	}
}

func TestCheckName(t *testing.T) {
	for _, name := range []string{"Inbox", "inbox", " INBOX ", ".Inbox", "Projects", "projects", " PROJECTS ", ".Projects"} {
		if err := CheckName(name); !errors.Is(err, ErrReservedName) {
			t.Errorf("CheckName(%q) = %v, want ErrReservedName", name, err)
		}
	}
	for _, name := range []string{"Dana", "Inbox Bot", "Inboxer", "Project", "Projects Lead"} {
		if err := CheckName(name); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", name, err)
		}
	}
}
