package staff

import (
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/store"
)

func TestCreateBot_StripsInvisibleCharactersAndRefusesAName(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PutOrg(t.Context(), org.Org{Name: "Acme", WorkspacePath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	s := &botService{opts: Deps{Store: db, Publish: func(any) {}}}
	for i, tc := range []struct{ name, in, want string }{
		{"bidi override", "a\u202eb", "ab"},
		{"zero-width space", "a\u200bb", "ab"},
		{"tag character", "a\U000E0041b", "ab"},
		{"variation selector", "a\ufe00b", "ab"},
		{"emoji presentation kept", "ok \u2764\ufe0f", "ok \u2764\ufe0f"},
	} {
		_, err := s.CreateBot(t.Context(), connect.NewRequest(&hearthv1.CreateBotRequest{
			Name: tc.in, Role: tc.in, JobDescription: tc.in, Tier: hearthv1.Tier_TIER_FAST,
		}))
		if tc.in != tc.want {
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("%s: a name with an invisible character gave %v, want invalid_argument", tc.name, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		// The same text with a plain name: role and job description are stripped.
		name := string(rune('A' + i))
		resp, err := s.CreateBot(t.Context(), connect.NewRequest(&hearthv1.CreateBotRequest{
			Name: name, Role: tc.in, JobDescription: tc.in, Tier: hearthv1.Tier_TIER_FAST,
		}))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if b := resp.Msg.GetBot(); b.GetRole() != tc.want || b.GetJobDescription() != tc.want {
			t.Errorf("%s: role %q, job %q, want %q", tc.name, b.GetRole(), b.GetJobDescription(), tc.want)
		}
	}
}
