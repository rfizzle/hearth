package provider

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/llm"
)

// sentinel stands in for a key; every output a test captures is searched for
// it.
const sentinel = "sk-test-sentinel"

type change struct {
	name    string
	deleted bool
}

type changes struct {
	mu  sync.Mutex
	got []change
}

func (c *changes) record(name string, deleted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, change{name, deleted})
}

func (c *changes) take() []change {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.got
	c.got = nil
	return out
}

func exists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	return err == nil
}

func names(cs []keystore.Credential) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

func TestSyncMovesFileKeysToTheAppThenDeletesTheFiles(t *testing.T) {
	Init()
	dir := filepath.Join(t.TempDir(), "Hearth")
	accts, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := accts.SetKey(Anthropic, sentinel+"-anthropic"); err != nil {
		t.Fatal(err)
	}
	if err := accts.Keys.Save("brave-search", sentinel+"-brave"); err != nil {
		t.Fatal(err)
	}
	var seen changes
	accts.OnCredentialChange = seen.record
	keysDir, chatDir := filepath.Join(dir, "keys"), filepath.Join(dir, "chatgpt")

	// The first sync: the app holds only one of them.
	toStore, err := accts.Sync([]keystore.Credential{{Name: "brave-search", Value: []byte("from-the-app")}})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(toStore); len(got) != 1 || got[0] != "anthropic" || string(toStore[0].Value) != sentinel+"-anthropic" {
		t.Fatalf("records to store = %v, want only anthropic", got)
	}
	if !exists(t, keysDir) {
		t.Fatal("the key files went on the first sync")
	}
	if got := seen.take(); len(got) != 0 {
		t.Errorf("loading the records fired %v", got)
	}
	// The app's copy won, and the account reads from memory.
	if v, _ := accts.Keys.Load("brave-search"); v != "from-the-app" {
		t.Errorf("brave-search = %q, want the app's", v)
	}
	if got := accts.List()[1]; got.State != SignedIn {
		t.Errorf("anthropic = %+v, want signed in from memory", got)
	}

	// A key set after the sync fires the change, is exported, and touches no
	// file.
	if err := accts.SetKey(Gemini, sentinel+"-gemini"); err != nil {
		t.Fatal(err)
	}
	if got := seen.take(); len(got) != 1 || got[0] != (change{"gemini", false}) {
		t.Errorf("changes = %v, want gemini stored", got)
	}
	rec, err := accts.Export("gemini")
	if err != nil || string(rec.Value) != sentinel+"-gemini" {
		t.Errorf("Export = %q, %v", rec.Value, err)
	}
	if exists(t, filepath.Join(keysDir, "gemini")) {
		t.Error("a key set after the sync was written to a file")
	}

	// The second sync holds everything the files held: they are deleted.
	if _, err := accts.Sync([]keystore.Credential{
		{Name: "brave-search", Value: []byte("x")}, {Name: "anthropic", Value: []byte("x")},
	}); err != nil {
		t.Fatal(err)
	}
	if exists(t, keysDir) || exists(t, chatDir) {
		t.Error("the files outlived the second sync")
	}

	// A removal is a deletion change and Export has nothing after it.
	if err := accts.ClearKey(Gemini); err != nil {
		t.Fatal(err)
	}
	if got := seen.take(); len(got) != 1 || got[0] != (change{"gemini", true}) {
		t.Errorf("changes = %v, want gemini deleted", got)
	}
	if _, err := accts.Export("gemini"); !errors.Is(err, keystore.ErrNoKey) {
		t.Errorf("Export after clear = %v, want keystore.ErrNoKey", err)
	}
}

func TestSyncMissingAFileHeldNameKeepsTheFiles(t *testing.T) {
	Init()
	dir := filepath.Join(t.TempDir(), "Hearth")
	accts, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = accts.Keys.Save("anthropic", sentinel+"-a")
	_ = accts.Keys.Save("brave-search", sentinel+"-b")
	if _, err := accts.Sync(nil); err != nil {
		t.Fatal(err)
	}
	keysDir := filepath.Join(dir, "keys")

	// One name short: the files stay, both keys intact.
	if _, err := accts.Sync([]keystore.Credential{{Name: "anthropic", Value: []byte("x")}}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"anthropic", "brave-search"} {
		if !exists(t, filepath.Join(keysDir, n)) {
			t.Errorf("the %s file went though the sync missed a name", n)
		}
	}
	// A name the person removed since is not missing.
	if err := accts.Keys.Clear("brave-search"); err != nil {
		t.Fatal(err)
	}
	if _, err := accts.Sync([]keystore.Credential{{Name: "anthropic", Value: []byte("x")}}); err != nil {
		t.Fatal(err)
	}
	if exists(t, keysDir) {
		t.Error("the files stayed though every name was held or removed")
	}
}

func TestSyncRefusesBadNamesWithoutEchoingValues(t *testing.T) {
	Init()
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = accts.Sync([]keystore.Credential{{Name: "../escape", Value: []byte(sentinel)}})
	if err == nil || strings.Contains(err.Error(), sentinel) {
		t.Errorf("Sync = %v, want a refusal without the value", err)
	}
	if accts.mem != nil {
		t.Error("a refused sync switched the accounts to memory")
	}
	if err := keystore.NewMemoryKeys().Save("chatgpt", sentinel); err == nil || strings.Contains(err.Error(), sentinel) {
		t.Errorf("saving a reserved name = %v", err)
	}
}

func TestSignInSavesToMemoryAndFiresTheChange(t *testing.T) {
	r := newRig(t)
	var seen changes
	r.accts.OnCredentialChange = seen.record
	if _, err := r.accts.Sync(nil); err != nil {
		t.Fatal(err)
	}
	page, err := r.accts.StartLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.auth.browse(r.l)(page); err != nil {
		t.Fatal(err)
	}
	for {
		if a := r.rec.next(t); a.State == SignedIn {
			break
		}
	}
	got := map[string]bool{}
	for _, c := range seen.take() {
		if c.deleted {
			t.Errorf("a sign-in deleted %q", c.name)
		}
		got[c.name] = true
	}
	if !got["chatgpt"] || !got["chatgpt-host"] {
		t.Errorf("changes = %v, want the record and the host id", got)
	}
	if exists(t, r.accts.fileChat.CredentialsPath()) {
		t.Error("the sign-in record was written to a file")
	}
	rec, err := r.accts.Export("chatgpt")
	if err != nil || !bytes.Contains(rec.Value, []byte("person@example.com")) {
		t.Errorf("Export(chatgpt) = %v", err)
	}
	if _, err := r.accts.Resolve(Balanced); err != nil {
		t.Errorf("Resolve from memory: %v", err)
	}
	// Logout rewrites the record in memory and fires the change again.
	seen.take()
	if _, err := r.accts.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := seen.take(); len(got) == 0 || got[0] != (change{"chatgpt", false}) {
		t.Errorf("logout changes = %v", got)
	}
}

func TestSignedOutWithNoAppAndNoFiles(t *testing.T) {
	Init()
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = accts.Resolve(Balanced)
	if !errors.Is(err, ErrNoAccount) {
		t.Fatalf("Resolve = %v, want no account", err)
	}
	if got := Explain(err); !strings.Contains(got, "hearthd login chatgpt") || strings.Contains(got, "\n") {
		t.Errorf("Explain = %q, want one plain line naming the command", got)
	}
}

func TestScrubReadsTheMemoryStore(t *testing.T) {
	Init()
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accts.Sync([]keystore.Credential{{Name: "brave-search", Value: []byte(sentinel + "-from-the-app")}}); err != nil {
		t.Fatal(err)
	}
	got := accts.Scrub()(llm.Message{Content: "the key is " + sentinel + "-from-the-app ok"})
	if strings.Contains(got.Content, sentinel) {
		t.Errorf("scrub left the key: %q", got.Content)
	}
}

func TestMemoryKeysNeverWritesAndNeverSpeaksAValue(t *testing.T) {
	m := keystore.NewMemoryKeys()
	var seen changes
	m.OnChange = seen.record
	if err := m.Save("anthropic", sentinel); err != nil {
		t.Fatal(err)
	}
	_ = m.SaveRecord([]byte(sentinel))
	_ = m.SaveHostID("urn:uuid:" + sentinel)
	_ = m.Clear("anthropic")
	for _, c := range seen.take() {
		if strings.Contains(c.name, sentinel) {
			t.Errorf("a change named a value: %v", c)
		}
	}
	if _, err := m.Load("nope"); !errors.Is(err, keystore.ErrNoKey) || strings.Contains(err.Error(), sentinel) {
		t.Errorf("Load = %v", err)
	}
	if m.Location() != "the app's Keychain" {
		t.Errorf("Location = %q", m.Location())
	}
	unlock, ok, _ := m.TryLock()
	if !ok {
		t.Fatal("TryLock refused a free lock")
	}
	if _, ok, _ := m.TryLock(); ok {
		t.Error("TryLock took a held lock")
	}
	unlock()
}
