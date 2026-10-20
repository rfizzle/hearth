package provider

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/llm"
)

// amazonBody is what a company gateway answered, with HTTP 200, to every
// path under a wrong address.
const amazonBody = `{"Output":{"__type":"com.amazon.coral.service#UnknownOperationException"},"Version":"1.0","service_tier":"default"}`

// answering is a client whose every request gets a 200 and body.
func answering(body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		_, _ = rec.WriteString(body)
		return rec.Result(), nil
	})}
}

func openWith(t *testing.T, c *http.Client) *Accounts {
	t.Helper()
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), c)
	if err != nil {
		t.Fatal(err)
	}
	return accts
}

func streamOnce(t *testing.T, accts *Accounts) (string, error) {
	t.Helper()
	m, err := accts.Resolve(Balanced)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := StreamFunc(m)(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil, "")
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	for ev := range ch {
		if ev.Err != nil {
			return out.String(), ev.Err
		}
		out.WriteString(ev.Token)
	}
	return out.String(), nil
}

func TestACompletionThatIsNotOneIsAnErrorWithTheBodysFirstWords(t *testing.T) {
	Init()
	accts := openWith(t, answering(amazonBody))
	if _, err := accts.SetProfile(Profile{Name: "Agoda gateway", BaseURL: "https://gw.example/claude", DefaultModel: "sonnet-5-5"}, gatewayKey); err != nil {
		t.Fatal(err)
	}
	got, err := streamOnce(t, accts)
	if err == nil {
		t.Fatalf("an empty reply from a foreign body was silence: %q", got)
	}
	want := `Agoda gateway answered, but not with a reply (it sent {"Output":{"__type":"com.amazon.coral.service#UnknownOperationException"},"Versi...`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}

func TestTheQuotedBodyHasNoKeyInIt(t *testing.T) {
	Init()
	accts := openWith(t, answering(`{"echo":"`+gatewayKey+`"}`))
	if _, err := accts.SetProfile(Profile{Name: "G", BaseURL: "https://gw.example/v1", DefaultModel: "m"}, gatewayKey); err != nil {
		t.Fatal(err)
	}
	_, err := streamOnce(t, accts)
	if err == nil {
		t.Fatal("no error for a body with no choices")
	}
	if strings.Contains(err.Error(), gatewayKey) || strings.Contains(err.Error(), "0123456789") {
		t.Errorf("the error carries the key: %v", err)
	}
}

func TestAnEmptyReplyThatHadChoicesIsNotAnError(t *testing.T) {
	Init()
	body := "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	accts := openWith(t, answering(body))
	if _, err := accts.SetProfile(Profile{Name: "G", BaseURL: "https://gw.example/v1", DefaultModel: "m"}, gatewayKey); err != nil {
		t.Fatal(err)
	}
	if got, err := streamOnce(t, accts); err != nil || got != "" {
		t.Errorf("got %q, %v; want an empty reply and no error", got, err)
	}
}

func TestSayPrintsWhatTheServiceSentInsteadOfAReply(t *testing.T) {
	Init()
	accts := openWith(t, answering(amazonBody))
	if _, err := accts.SetProfile(Profile{Name: "Agoda gateway", BaseURL: "https://gw.example/claude", DefaultModel: "m"}, gatewayKey); err != nil {
		t.Fatal(err)
	}
	m, _ := accts.Resolve(Balanced)
	var out bytes.Buffer
	err := Say(context.Background(), m.Provider, "hi", &out)
	if err == nil || !strings.Contains(Explain(err), "UnknownOperationException") {
		t.Errorf("Say = %v; want the service's words", err)
	}
	if out.Len() != 0 {
		t.Errorf("Say printed %q before failing", out.String())
	}
}
