package inbox

import (
	"errors"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func grantsFixture() []org.Grant {
	return []org.Grant{
		{BotID: "b", Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: "/w/Dana"},
		{BotID: "b", Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: "/w/Dana"},
		{BotID: "b", Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: "/w/Other"},
		{BotID: "b", Capability: org.FilesDelete, Mode: org.ModeAsk, Folder: "/w/Dana"},
		{BotID: "b", Capability: org.WebFetch, Mode: org.ModeAsk},
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name       string
		decision   Decision
		capability org.Capability
		folder     string
		wantErr    error
		wantAllow  *Allowance
		wantModes  []org.Mode // the mode of each fixture grant afterwards
	}{
		{"always allow raises the covering grant only", AlwaysAllow, org.FilesWrite, "/w/Dana/notes", nil, nil,
			[]org.Mode{2, 2, 1, 1, 1}},
		{"always allow with no folder raises a folderless grant", AlwaysAllow, org.WebFetch, "", nil, nil,
			[]org.Mode{2, 1, 1, 1, 2}},
		{"allow for task returns an allowance and changes no grant", AllowForTask, org.FilesWrite, "/w/Dana",
			nil, &Allowance{Capability: org.FilesWrite, Folder: "/w/Dana"}, []org.Mode{2, 1, 1, 1, 1}},
		{"allow for task works for a delete", AllowForTask, org.FilesDelete, "/w/Dana",
			nil, &Allowance{Capability: org.FilesDelete, Folder: "/w/Dana"}, []org.Mode{2, 1, 1, 1, 1}},
		{"always allow of a delete has no standing", AlwaysAllow, org.FilesDelete, "/w/Dana", ErrNoStanding, nil,
			[]org.Mode{2, 1, 1, 1, 1}},
		{"always allow of a question has no standing", AlwaysAllow, org.AskOwner, "", ErrNoStanding, nil,
			[]org.Mode{2, 1, 1, 1, 1}},
		{"allow once changes nothing", AllowOnce, org.FilesWrite, "/w/Dana", nil, nil, []org.Mode{2, 1, 1, 1, 1}},
		{"deny changes nothing", Deny, org.FilesWrite, "/w/Dana", nil, nil, []org.Mode{2, 1, 1, 1, 1}},
		{"a sibling folder with the same prefix is not covered", AlwaysAllow, org.FilesWrite, "/w/DanaTwo", nil, nil,
			[]org.Mode{2, 1, 1, 1, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := grantsFixture()
			got, allow, err := Apply(tt.decision, in, tt.capability, tt.folder)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if (allow == nil) != (tt.wantAllow == nil) || (allow != nil && *allow != *tt.wantAllow) {
				t.Fatalf("allowance = %+v, want %+v", allow, tt.wantAllow)
			}
			var modes []org.Mode
			for _, g := range got {
				modes = append(modes, g.Mode)
			}
			if !slices.Equal(modes, tt.wantModes) {
				t.Fatalf("modes = %v, want %v", modes, tt.wantModes)
			}
			if !slices.Equal(in, grantsFixture()) {
				t.Fatal("Apply changed the grants it was given")
			}
		})
	}
}

func TestCheckGrants_RefusesTheFoldersHearthKeepsItsOwnFilesIn(t *testing.T) {
	reserved := []string{"/Users/me/Library/Application Support/hearth", "/private/data"}
	tests := []struct {
		folder  string
		wantErr error
	}{
		{"/Users/me", ErrReservedFolder},
		{"/Users/me/Library", ErrReservedFolder},
		{"/Users/me/Library/Application Support/hearth", ErrReservedFolder},
		{"/Users/me/Library/Application Support/hearth/keys", ErrReservedFolder},
		{"/private/data/x", ErrReservedFolder},
		{"/Users/me/Documents", nil},
		{"/Users/me/Library/Application Support/hearth2", nil},
	}
	for _, tt := range tests {
		t.Run(tt.folder, func(t *testing.T) {
			err := CheckGrants([]org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: tt.folder}}, reserved...)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	if err := CheckGrants([]org.Grant{{Capability: org.WebFetch, Mode: org.ModeAllowed}}, reserved...); err != nil {
		t.Errorf("a grant with no folder = %v", err)
	}
}

func TestStandingAndCheckGrants(t *testing.T) {
	for c, want := range map[org.Capability]bool{
		org.FilesRead: true, org.FilesWrite: true, org.WebFetch: true, org.WebSearch: true,
		org.FilesDelete: false, org.AskOwner: false,
	} {
		if got := Standing(c); got != want {
			t.Errorf("Standing(%s) = %v, want %v", c, got, want)
		}
	}
	tests := []struct {
		name    string
		grants  []org.Grant
		wantErr error
	}{
		{"nothing", nil, nil},
		{"standing allowed", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: "/w"}}, nil},
		{"delete asks", []org.Grant{{Capability: org.FilesDelete, Mode: org.ModeAsk, Folder: "/w"}}, nil},
		{"delete off", []org.Grant{{Capability: org.FilesDelete, Mode: org.ModeNotGranted, Folder: "/w"}}, nil},
		{"delete allowed", []org.Grant{{Capability: org.FilesDelete, Mode: org.ModeAllowed, Folder: "/w"}}, ErrNoStanding},
		{"a relative folder", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAsk, Folder: "Dana"}}, ErrBadFolder},
		{"a folder that climbs", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAsk, Folder: "/w/Dana/../Sam"}}, ErrBadFolder},
		{"a folder with a trailing slash", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAsk, Folder: "/w/Dana/"}}, ErrBadFolder},
		{"the right to ask is always allowed", []org.Grant{{Capability: org.AskOwner, Mode: org.ModeAllowed}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckGrants(tt.grants); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
