package rpc

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/prototext"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

const sentinel = "sk-test-sentinel"

// The handshake end to end: a key set after the sync reaches the bus as a
// name, and only ExportCredential hands the value back. Every event, error
// and log line the run produced is searched for the value.
func TestCredentials_OnlyExportCarriesAValue(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	f := follow(t, c, r.bus)

	// The first sync: the app holds one record and the core none.
	sync := unary(t, c.prov.SyncCredentials, &hearthv1.SyncCredentialsRequest{
		Records: []*hearthv1.Credential{{Name: "brave-search", Value: []byte(sentinel + "-app")}},
	})
	if len(sync.GetRecordsToStore()) != 0 {
		t.Errorf("records_to_store = %v, want none", sync.GetRecordsToStore())
	}

	// A key set after the sync.
	unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{Provider: hearthv1.Provider_PROVIDER_ANTHROPIC, Key: sentinel})
	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetCredentialChanged() != nil })
	got := evs[len(evs)-1].GetCredentialChanged()
	if got.GetName() != "anthropic" || got.GetDeleted() {
		t.Errorf("credential_changed = %v, want anthropic stored", got)
	}

	exp := unary(t, c.prov.ExportCredential, &hearthv1.ExportCredentialRequest{Name: "anthropic"})
	if exp.GetRecord().GetName() != "anthropic" || string(exp.GetRecord().GetValue()) != sentinel {
		t.Errorf("ExportCredential = %v", exp.GetRecord().GetName())
	}

	_, missErr := c.prov.ExportCredential(t.Context(), connect.NewRequest(&hearthv1.ExportCredentialRequest{Name: "gemini"}))
	if connect.CodeOf(missErr) != connect.CodeNotFound {
		t.Errorf("export of nothing = %v, want not found", missErr)
	}
	_, badErr := c.prov.SyncCredentials(t.Context(), connect.NewRequest(&hearthv1.SyncCredentialsRequest{
		Records: []*hearthv1.Credential{{Name: "../x", Value: []byte(sentinel)}},
	}))
	if connect.CodeOf(badErr) != connect.CodeInvalidArgument {
		t.Errorf("bad name = %v, want invalid argument", badErr)
	}

	unary(t, c.prov.ClearCredential, &hearthv1.ClearCredentialRequest{Provider: hearthv1.Provider_PROVIDER_ANTHROPIC})
	evs = append(evs, f.until(func(m *hearthv1.SubscribeResponse) bool {
		return m.GetCredentialChanged().GetDeleted()
	})...)

	out := logs.String() + missErr.Error() + badErr.Error() + prototext.Format(sync)
	for _, e := range evs {
		out += prototext.Format(e)
	}
	if strings.Contains(out, sentinel) {
		t.Errorf("an event, error or log line carries a credential value:\n%s", out)
	}
}
