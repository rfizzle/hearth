package wire

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/intro"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/store"
)

func TestProviderNames_RoundTrip(t *testing.T) {
	for e, n := range ProviderNames {
		if got := ToProto(ProviderNames, n); got != e {
			t.Errorf("ToProto(%q) = %v, want %v", n, got, e)
		}
		if got, err := FromProto(ProviderNames, e); err != nil || got != n {
			t.Errorf("FromProto(%v) = %q, %v; want %q", e, got, err, n)
		}
	}
	if _, err := FromProto(ProviderNames, hearthv1.Provider_PROVIDER_UNSPECIFIED); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("FromProto(unspecified) = %v, want invalid_argument", err)
	}
}

func TestTierFromProto(t *testing.T) {
	if got, err := TierFromProto(hearthv1.Tier_TIER_BEST); err != nil || got != org.TierBest {
		t.Errorf("TierFromProto(best) = %v, %v", got, err)
	}
	if _, err := TierFromProto(hearthv1.Tier_TIER_UNSPECIFIED); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("TierFromProto(unspecified) = %v, want invalid_argument", err)
	}
}

func TestFail_PicksTheCode(t *testing.T) {
	tests := []struct {
		err  error
		want connect.Code
	}{
		{fmt.Errorf("bot: %w", store.ErrNotFound), connect.CodeNotFound},
		{Unavailable(errors.New("sign in")), connect.CodeUnavailable},
		{Invalid("x is required"), connect.CodeInvalidArgument},
		{errors.New("disk full"), connect.CodeInternal},
	}
	for _, tt := range tests {
		if got := connect.CodeOf(Fail(tt.err)); got != tt.want {
			t.Errorf("Fail(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
	if err := Need("name", "Dana", "role", " "); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Need with an empty role = %v, want invalid_argument", err)
	}
}

func TestReplyAndEach(t *testing.T) {
	if got := Each([]int{1, 2}, func(i int) string { return fmt.Sprint(i * 2) }); len(got) != 2 || got[1] != "4" {
		t.Errorf("Each = %v", got)
	}
	if res, err := Reply(&hearthv1.PingResponse{Version: "1"}, nil); err != nil || res.Msg.GetVersion() != "1" {
		t.Errorf("Reply(ok) = %v, %v", res, err)
	}
	if _, err := Reply(&hearthv1.PingResponse{}, store.ErrNotFound); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Reply(not found) = %v, want not_found", err)
	}
}

func TestConversions(t *testing.T) {
	if Timestamp(time.Time{}) != nil {
		t.Error("the zero time is not nil")
	}
	recs := CredentialsFromProto([]*hearthv1.Credential{{Name: "anthropic", Value: []byte("k")}})
	if len(recs) != 1 || recs[0].Name != "anthropic" || string(recs[0].Value) != "k" {
		t.Fatalf("CredentialsFromProto = %+v", recs)
	}
	if c := CredentialToProto(recs[0]); c.GetName() != "anthropic" || string(c.GetValue()) != "k" {
		t.Errorf("CredentialToProto = %v", c)
	}
	if b := BotToProto(org.Bot{ID: "b1", Tier: org.TierFast}); b.GetId() != "b1" || b.GetTier() != hearthv1.Tier_TIER_FAST {
		t.Errorf("BotToProto = %v", b)
	}
	in := intro.Assemble("Writer", "Writes.")
	got := BotToProto(org.Bot{Introduction: in}).GetIntroduction()
	if got.GetPurpose() != in.Purpose || len(got.GetHow()) != 3 || got.GetHow()[1].GetSymbol() != "folder" ||
		got.GetHow()[1].GetText() != in.How[1].Text || len(got.GetTryAsking()) != 3 || got.GetTryAsking()[2] != in.Try[2] {
		t.Errorf("IntroductionToProto = %v, want %+v", got, in)
	}
}
