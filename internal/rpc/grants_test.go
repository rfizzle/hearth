package rpc

import (
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func row(g *hearthv1.Grant) [3]any {
	return [3]any{g.GetCapability(), g.GetFolder(), g.GetMode()}
}

func TestListGrants_OneRowPerCapabilityAndFolderWithTheOwnFolderMarked(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	own := bot.GetFolderPath()

	got := unary(t, c.bot.ListGrants, &hearthv1.ListGrantsRequest{BotId: bot.GetId()}).GetGrants()
	var rows [][3]any
	for _, g := range got {
		rows = append(rows, row(g))
	}
	want := [][3]any{
		{hearthv1.Capability_CAPABILITY_FILES_READ, own, hearthv1.GrantMode_GRANT_MODE_ALLOWED},
		{hearthv1.Capability_CAPABILITY_FILES_WRITE, own, hearthv1.GrantMode_GRANT_MODE_ALLOWED},
		{hearthv1.Capability_CAPABILITY_FILES_DELETE, own, hearthv1.GrantMode_GRANT_MODE_ASK},
		{hearthv1.Capability_CAPABILITY_WEB_FETCH, "", hearthv1.GrantMode_GRANT_MODE_ALLOWED},
		{hearthv1.Capability_CAPABILITY_WEB_SEARCH, "", hearthv1.GrantMode_GRANT_MODE_ALLOWED},
	}
	if len(rows) != len(want) {
		t.Fatalf("ListGrants = %v, want %v", got, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %v, want %v", i, rows[i], want[i])
		}
	}
	for _, g := range got {
		isDelete := g.GetCapability() == hearthv1.Capability_CAPABILITY_FILES_DELETE
		if g.GetTitle() == "" || g.GetCanAllow() == isDelete {
			t.Errorf("%v: title %q, can_allow %v; only delete may not be allowed", g.GetCapability(), g.GetTitle(), g.GetCanAllow())
		}
		if g.GetOwnFolder() != (g.GetFolder() == own) {
			t.Errorf("%v: own_folder %v for folder %q", g.GetCapability(), g.GetOwnFolder(), g.GetFolder())
		}
	}
	if _, err := c.bot.ListGrants(t.Context(), connect.NewRequest(&hearthv1.ListGrantsRequest{BotId: "nope"})); code(err) != connect.CodeNotFound {
		t.Errorf("ListGrants of an unknown bot = %v, want not_found", err)
	}
	if _, err := c.bot.ListGrants(t.Context(), connect.NewRequest(&hearthv1.ListGrantsRequest{})); code(err) != connect.CodeInvalidArgument {
		t.Errorf("ListGrants with no bot = %v, want invalid_argument", err)
	}
}

func TestSetGrants_KeepsTheOwnFolderAndTheRightToAskAndTellsTheBus(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	own, shared := bot.GetFolderPath(), filepath.Join(r.root, "Shared")
	f := follow(t, c, r.bus)

	// The sheet's save, with the own folder and the right to ask both left out.
	res := unary(t, c.bot.SetGrants, &hearthv1.SetGrantsRequest{BotId: bot.GetId(), Grants: []*hearthv1.Grant{
		{Capability: hearthv1.Capability_CAPABILITY_FILES_READ, Mode: hearthv1.GrantMode_GRANT_MODE_ALLOWED, Folder: shared},
		{Capability: hearthv1.Capability_CAPABILITY_WEB_FETCH, Mode: hearthv1.GrantMode_GRANT_MODE_NOT_GRANTED},
	}})
	got := map[[2]any]*hearthv1.Grant{}
	for _, g := range res.GetGrants() {
		got[[2]any{g.GetCapability(), g.GetFolder()}] = g
	}
	for _, key := range [][2]any{
		{hearthv1.Capability_CAPABILITY_FILES_READ, own}, {hearthv1.Capability_CAPABILITY_FILES_WRITE, own},
		{hearthv1.Capability_CAPABILITY_FILES_DELETE, own}, {hearthv1.Capability_CAPABILITY_FILES_READ, shared},
	} {
		if g := got[key]; g == nil || g.GetOwnFolder() != (key[1] == own) {
			t.Errorf("SetGrants left no row for %v: %v", key, res.GetGrants())
		}
	}
	if g := got[[2]any{hearthv1.Capability_CAPABILITY_WEB_SEARCH, ""}]; g.GetMode() != hearthv1.GrantMode_GRANT_MODE_NOT_GRANTED {
		t.Errorf("web_search was left out and is %v, want not granted", g)
	}
	if g := got[[2]any{hearthv1.Capability_CAPABILITY_WEB_FETCH, ""}]; g.GetMode() != hearthv1.GrantMode_GRANT_MODE_NOT_GRANTED {
		t.Errorf("web_fetch = %v, want not granted", g)
	}

	stored, err := r.opts.Store.GrantsFor(t.Context(), bot.GetId())
	if err != nil {
		t.Fatal(err)
	}
	if org.Allowed(stored, org.AskOwner) != org.ModeAllowed {
		t.Errorf("stored grants %v lost the right to ask the owner", stored)
	}

	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetGrantsChanged() != nil })
	if ch := evs[len(evs)-1].GetGrantsChanged(); ch.GetBotId() != bot.GetId() || len(ch.GetGrants()) != len(res.GetGrants()) {
		t.Errorf("grants_changed = %v, want the bot's grants as answered", ch)
	}
}

func TestSetGrants_RefusesWhatItMust(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	bot := hireDana(t, c, r)
	own := bot.GetFolderPath()
	before := unary(t, c.bot.ListGrants, &hearthv1.ListGrantsRequest{BotId: bot.GetId()}).GetGrants()

	grant := func(c hearthv1.Capability, m hearthv1.GrantMode, folder ...string) []*hearthv1.Grant {
		return []*hearthv1.Grant{{Capability: c, Mode: m, Folder: append(folder, own)[0]}}
	}
	tests := []struct {
		name   string
		botID  string
		grants []*hearthv1.Grant
		want   connect.Code
	}{
		{"allowed on delete", bot.GetId(), grant(hearthv1.Capability_CAPABILITY_FILES_DELETE, hearthv1.GrantMode_GRANT_MODE_ALLOWED), connect.CodeInvalidArgument},
		{"a capability the contract leaves unspecified", bot.GetId(), grant(hearthv1.Capability_CAPABILITY_UNSPECIFIED, hearthv1.GrantMode_GRANT_MODE_ASK), connect.CodeInvalidArgument},
		{"a folder that is not a full path", bot.GetId(), []*hearthv1.Grant{{Capability: hearthv1.Capability_CAPABILITY_FILES_READ, Mode: hearthv1.GrantMode_GRANT_MODE_ASK, Folder: "../etc"}}, connect.CodeInvalidArgument},
		{"a mode left unspecified", bot.GetId(), grant(hearthv1.Capability_CAPABILITY_FILES_READ, hearthv1.GrantMode_GRANT_MODE_UNSPECIFIED), connect.CodeInvalidArgument},
		{"the folder hearth keeps its files in", bot.GetId(), grant(hearthv1.Capability_CAPABILITY_FILES_WRITE, hearthv1.GrantMode_GRANT_MODE_ASK, r.opts.ReservedFolders()[0]), connect.CodeInvalidArgument},
		{"a folder that holds it", bot.GetId(), grant(hearthv1.Capability_CAPABILITY_FILES_WRITE, hearthv1.GrantMode_GRANT_MODE_ASK, filepath.Dir(r.opts.ReservedFolders()[0])), connect.CodeInvalidArgument},
		{"no bot named", "", nil, connect.CodeInvalidArgument},
		{"an unknown bot", "nope", grant(hearthv1.Capability_CAPABILITY_FILES_READ, hearthv1.GrantMode_GRANT_MODE_ASK), connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.bot.SetGrants(t.Context(), connect.NewRequest(&hearthv1.SetGrantsRequest{BotId: tt.botID, Grants: tt.grants}))
			if code(err) != tt.want {
				t.Errorf("SetGrants = %v, want %v", err, tt.want)
			}
		})
	}
	after := unary(t, c.bot.ListGrants, &hearthv1.ListGrantsRequest{BotId: bot.GetId()}).GetGrants()
	if len(after) != len(before) {
		t.Errorf("refused edits changed the grants: %v, then %v", before, after)
	}
	for i := range before {
		if row(before[i]) != row(after[i]) {
			t.Errorf("row %d changed from %v to %v", i, row(before[i]), row(after[i]))
		}
	}
}
