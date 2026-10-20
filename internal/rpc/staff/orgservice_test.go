package staff

import (
	"testing"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func TestUpdateOrg_RefusesAnInvisibleCharacterInTheName(t *testing.T) {
	s := &orgService{opts: Deps{}}
	for _, name := range []string{"Ac\u200bme", "Acme\u202e", "Ac\u2060me"} {
		_, err := s.UpdateOrg(t.Context(), connect.NewRequest(&hearthv1.UpdateOrgRequest{Name: name, WorkspacePath: "/Hearth"}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%q: UpdateOrg = %v, want invalid_argument", name, err)
		}
	}
}

func TestUpdateOrg_RefusesWhileNoWayToMakeTheFolderIsSet(t *testing.T) {
	s := &orgService{opts: Deps{}}
	_, err := s.UpdateOrg(t.Context(), connect.NewRequest(&hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: "/Hearth"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("UpdateOrg = %v, want failed_precondition", err)
	}
}
