package staff

import (
	"testing"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func TestUpdateOrg_RefusesWhileNoWayToMakeTheFolderIsSet(t *testing.T) {
	s := &orgService{opts: Deps{}}
	_, err := s.UpdateOrg(t.Context(), connect.NewRequest(&hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: "/Hearth"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("UpdateOrg = %v, want failed_precondition", err)
	}
}
