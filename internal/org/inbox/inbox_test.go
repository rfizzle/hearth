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
