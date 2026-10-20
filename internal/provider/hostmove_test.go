package provider

import (
	"strings"
	"testing"
)

func TestSetProfileWontCarryAKeyToAnotherHost(t *testing.T) {
	accts := openWith(t, nil)
	acct, err := accts.SetProfile(Profile{Name: "Acme", BaseURL: "https://llm.acme.example/v1", DefaultModel: "m"}, gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	moved := Profile{ID: acct.Profile, Name: "Acme", BaseURL: "https://evil.example/v1", DefaultModel: "m"}
	if _, err := accts.SetProfile(moved, ""); err == nil || err.Error() != "the address changed; paste the key again" {
		t.Errorf("a new host with no key = %v, want the plain refusal", err)
	}
	if _, err := accts.SaveProfile(t.Context(), moved, ""); err == nil || !strings.Contains(err.Error(), "paste the key again") {
		t.Errorf("SaveProfile to a new host with no key = %v", err)
	}
	if got := accts.Profiles(); len(got) != 1 || got[0].BaseURL != "https://llm.acme.example/v1" {
		t.Errorf("the refused change was stored: %+v", got)
	}
	// A new name, model or path on the same host keeps the key; a new key
	// goes with a new host.
	same := Profile{ID: acct.Profile, Name: "Acme 2", BaseURL: "https://llm.acme.example/v2", DefaultModel: "m2"}
	if _, err := accts.SetProfile(same, ""); err != nil {
		t.Errorf("a change on the same host with no key = %v", err)
	}
	moved.BaseURL = "https://other.acme.example/v1"
	if _, err := accts.SetProfile(moved, "new-key-0123456789"); err != nil {
		t.Errorf("a new host with a key = %v", err)
	}
}
