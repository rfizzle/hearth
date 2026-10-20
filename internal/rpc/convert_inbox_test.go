package rpc

import (
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// The tables hold every domain value, and every contract value but
// unspecified, and the two directions agree. A value added on either side
// without a row fails here.

func TestEnumTables_EveryDomainValueHasAContractValueAndBack(t *testing.T) {
	// The right to ask the owner is the one capability the contract has no value for.
	for _, c := range []org.Capability{org.FilesRead, org.FilesWrite, org.FilesDelete, org.WebFetch, org.WebSearch} {
		e := wire.ToProto(capabilities, c)
		if e == hearthv1.Capability_CAPABILITY_UNSPECIFIED {
			t.Errorf("capability %s has no contract value", c)
		} else if back, err := wire.FromProto(capabilities, e); err != nil || back != c {
			t.Errorf("capability %s -> %v -> %v, %v", c, e, back, err)
		}
	}
	for _, m := range []org.Mode{org.ModeNotGranted, org.ModeAsk, org.ModeAllowed} {
		e := wire.ToProto(grantModes, m)
		if e == hearthv1.GrantMode_GRANT_MODE_UNSPECIFIED {
			t.Errorf("mode %d has no contract value", m)
		} else if back, err := wire.FromProto(grantModes, e); err != nil || back != m {
			t.Errorf("mode %d -> %v -> %v, %v", m, e, back, err)
		}
	}
	for _, d := range []inbox.Decision{inbox.AllowOnce, inbox.AllowForTask, inbox.AlwaysAllow, inbox.Deny} {
		e := wire.ToProto(decisions, d)
		if e == hearthv1.Decision_DECISION_UNSPECIFIED {
			t.Errorf("decision %d has no contract value", d)
		} else if back, err := wire.FromProto(decisions, e); err != nil || back != d {
			t.Errorf("decision %d -> %v -> %v, %v", d, e, back, err)
		}
	}
	for _, k := range []inbox.Kind{inbox.Approval, inbox.Question, inbox.Failure, inbox.Budget} {
		e := wire.ToProto(inboxKinds, k)
		if e == hearthv1.InboxKind_INBOX_KIND_UNSPECIFIED {
			t.Errorf("kind %d has no contract value", k)
		} else if back, err := wire.FromProto(inboxKinds, e); err != nil || back != k {
			t.Errorf("kind %d -> %v -> %v, %v", k, e, back, err)
		}
	}
	for _, s := range []inbox.State{inbox.Open, inbox.Closed} {
		e := wire.ToProto(inboxStates, s)
		if e == hearthv1.InboxState_INBOX_STATE_UNSPECIFIED {
			t.Errorf("state %d has no domain value", s)
		} else if back, err := wire.FromProto(inboxStates, e); err != nil || back != s {
			t.Errorf("state %d -> %v -> %v, %v", s, e, back, err)
		}
	}
}

func TestEnumTables_EveryContractValueHasADomainValue(t *testing.T) {
	for v := range hearthv1.Capability_name {
		if e := hearthv1.Capability(v); e != hearthv1.Capability_CAPABILITY_UNSPECIFIED {
			if _, ok := capabilities[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.GrantMode_name {
		if e := hearthv1.GrantMode(v); e != hearthv1.GrantMode_GRANT_MODE_UNSPECIFIED {
			if _, ok := grantModes[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.Decision_name {
		if e := hearthv1.Decision(v); e != hearthv1.Decision_DECISION_UNSPECIFIED {
			if _, ok := decisions[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.InboxKind_name {
		if e := hearthv1.InboxKind(v); e != hearthv1.InboxKind_INBOX_KIND_UNSPECIFIED {
			if _, ok := inboxKinds[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for v := range hearthv1.InboxState_name {
		if e := hearthv1.InboxState(v); e != hearthv1.InboxState_INBOX_STATE_UNSPECIFIED {
			if _, ok := inboxStates[e]; !ok {
				t.Errorf("%v has no domain value", e)
			}
		}
	}
	for _, row := range capabilityRows {
		if _, ok := capabilities[row.capability]; !ok || row.title == "" {
			t.Errorf("sheet row %v has no domain value or no title", row.capability)
		}
	}
	if len(capabilityRows) != len(capabilities) {
		t.Errorf("the sheet lists %d capabilities and the table has %d", len(capabilityRows), len(capabilities))
	}
}
