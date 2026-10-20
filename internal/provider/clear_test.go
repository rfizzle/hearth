package provider

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rfizzle/llm"
)

func TestClear_RemovesAKeyAndSignsOutOfChatGPT(t *testing.T) {
	Init()
	a, err := Open(filepath.Join(t.TempDir(), "Hearth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetKey(Anthropic, "sk-ant-test-key-1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.Clear(t.Context(), Anthropic); err != nil {
		t.Fatalf("Clear(Anthropic): %v", err)
	}
	for _, acct := range a.List() {
		if acct.Provider == Anthropic && acct.State != SignedOut {
			t.Errorf("Anthropic after Clear = %v, want signed out", acct.State)
		}
	}
	// ChatGPT has no key: Clear reaches Logout, which says nobody is signed in.
	if err := a.Clear(t.Context(), ChatGPT); !errors.Is(err, llm.ErrChatGPTSignedOut) {
		t.Errorf("Clear(ChatGPT) = %v, want the signed-out error from Logout", err)
	}
}
