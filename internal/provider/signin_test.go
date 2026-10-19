package provider

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"
)

func TestSignInPersistsAndSignsOut(t *testing.T) {
	Init()
	dir := filepath.Join(t.TempDir(), "Hearth")
	auth := newFakeAuth(t)
	ctx := context.Background()

	// Before anyone signs in, nothing resolves and the words name the command.
	accts, err := Open(dir, auth.client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = accts.Resolve(Balanced)
	if !errors.Is(err, ErrNoAccount) {
		t.Fatalf("Resolve before sign-in = %v, want no account", err)
	}
	if got := Explain(err); !strings.Contains(got, "hearthd login chatgpt") || strings.Contains(got, "\n") {
		t.Errorf("Explain = %q, want one line naming the command", got)
	}

	// Sign in through the fake browser and the in-memory loopback.
	l := newMemListener()
	var said bytes.Buffer
	err = accts.Login(ctx, chatgpt.LoginOptions{
		Browser: auth.browse(l), Say: &said,
		Listen: func() (net.Listener, error) { return l, nil }, Wait: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !strings.Contains(said.String(), "person@example.com") {
		t.Errorf("sign-in said %q, want the account", said.String())
	}

	// The directory is the owner's alone and so are its files.
	store := filepath.Join(dir, "chatgpt")
	info, err := os.Stat(store)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}
	entries, err := os.ReadDir(store)
	if err != nil || len(entries) == 0 {
		t.Fatalf("store is empty: %v", err)
	}
	for _, e := range entries {
		if fi, err := e.Info(); err != nil || fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode = %v, %v; want owner only", e.Name(), fi.Mode().Perm(), err)
		}
	}

	// A relaunch is new accounts over the same folder: signed in without the
	// browser, and the tiers resolve to the plan's defaults.
	again, err := Open(dir, auth.client())
	if err != nil {
		t.Fatal(err)
	}
	if got := again.List()[0]; got.Provider != ChatGPT || got.State != SignedIn || got.Label != "person@example.com" {
		t.Errorf("List()[0] after relaunch = %+v", got)
	}
	d := llm.Defaults("chatgpt")
	for tier, want := range map[Tier]string{Fast: d.CheapModel, Balanced: d.Model, Best: d.Model} {
		m, err := again.Resolve(tier)
		if err != nil || m.Provider.Name() != "chatgpt" || m.Name != want || m.Account != ChatGPT {
			t.Errorf("Resolve(%d) = %+v, %v; want chatgpt %q", tier, m, err, want)
		}
	}

	// Sign out revokes at the server and leaves the person signed out.
	revoked, err := again.Logout(ctx)
	if err != nil || !revoked || auth.revoked != 1 {
		t.Fatalf("Logout = %v, %v (server saw %d)", revoked, err, auth.revoked)
	}
	if _, err := again.Resolve(Balanced); !errors.Is(err, ErrNoAccount) {
		t.Errorf("Resolve after sign-out = %v, want no account", err)
	}
	if _, err := again.Logout(ctx); !errors.Is(err, llm.ErrChatGPTSignedOut) {
		t.Errorf("second Logout = %v, want signed out", err)
	}
}
