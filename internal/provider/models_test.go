package provider

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// modelsService answers the models path the way the table says and
// remembers what it was asked.
func modelsService(status int, body string, seen *[]string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*seen = append(*seen, r.URL.Path+" "+r.Header.Get("Authorization"))
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(status)
		_, _ = rec.WriteString(body)
		return rec.Result(), nil
	})}
}

func TestSaveProfileChecksTheAddressFirst(t *testing.T) {
	Init()
	p := Profile{Name: "Acme gateway", BaseURL: "https://gw.example/v1/", DefaultModel: "claude-sonnet-5-5"}
	tests := []struct {
		name   string
		status int
		body   string
		want   string // empty means the save succeeds
	}{
		{"a model list", 200, `{"object":"list","data":[{"id":"claude-sonnet-5-5"},{"id":"gpt-x"}]}`, ""},
		{"an empty model list", 200, `{"data":[]}`, ""},
		{"the foreign body", 200, amazonBody, "not as an OpenAI-compatible service"},
		{"not json", 200, `<html>sign in</html>`, "many gateways need `/v1`"},
		{"no such path", 404, `{"message":"no route"}`, "not as an OpenAI-compatible service"},
		{"key refused", 401, `{"error":"bad key ` + gatewayKey + `"}`, "The key was refused."},
		{"key forbidden", 403, ``, "The key was refused."},
		{"server error", 500, `oops`, "not as an OpenAI-compatible service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen []string
			accts := openWith(t, modelsService(tt.status, tt.body, &seen))
			acct, err := accts.SaveProfile(t.Context(), p, " "+gatewayKey+" ")
			if want := []string{"/v1/models Bearer " + gatewayKey}; !slices.Equal(seen, want) {
				t.Errorf("the service saw %q, want %q", seen, want)
			}
			if tt.want == "" {
				if err != nil || acct.Profile == "" || acct.BaseURL != "https://gw.example/v1" {
					t.Fatalf("SaveProfile = %+v, %v; want it saved", acct, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("SaveProfile error = %v, want it to contain %q", err, tt.want)
			}
			if strings.Contains(err.Error(), gatewayKey) {
				t.Errorf("the refusal carries the key: %v", err)
			}
			if got := accts.Profiles(); len(got) != 0 {
				t.Errorf("a refused save stored %+v", got)
			}
			if names, _ := accts.Keys.Names(); len(names) != 0 {
				t.Errorf("a refused save left %v behind", names)
			}
		})
	}
}

func TestSaveProfileUnreachable(t *testing.T) {
	Init()
	down := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, http.ErrHandlerTimeout })}
	accts := openWith(t, down)
	_, err := accts.SaveProfile(t.Context(), Profile{Name: "G", BaseURL: "https://gw.example/v1", DefaultModel: "m"}, gatewayKey)
	if err == nil || err.Error() != "Could not reach that address." {
		t.Errorf("SaveProfile error = %v", err)
	}
}

func TestSaveProfileEditChecksWithTheStoredKey(t *testing.T) {
	Init()
	var seen []string
	accts := openWith(t, modelsService(200, `{"data":[{"id":"m"}]}`, &seen))
	first, err := accts.SaveProfile(t.Context(), Profile{Name: "G", BaseURL: "https://gw.example/v1", DefaultModel: "m"}, gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	seen = nil
	if _, err := accts.SaveProfile(t.Context(), Profile{ID: first.Profile, Name: "G2", BaseURL: "https://gw.example/v2", DefaultModel: "m"}, ""); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/v2/models Bearer " + gatewayKey}; !slices.Equal(seen, want) {
		t.Errorf("the edit asked %q, want %q", seen, want)
	}
}

func TestSaveProfileLeavesFieldErrorsToSetProfile(t *testing.T) {
	Init()
	var seen []string
	accts := openWith(t, modelsService(200, `{"data":[]}`, &seen))
	// No key for a service on the internet, and a missing model: said
	// before anything is asked of the network.
	for _, p := range []Profile{
		{Name: "G", BaseURL: "https://gw.example/v1", DefaultModel: "m"},
		{Name: "G", BaseURL: "file:///etc/passwd", DefaultModel: "m"},
	} {
		if _, err := accts.SaveProfile(t.Context(), p, ""); err == nil {
			t.Errorf("SaveProfile(%+v) succeeded", p)
		}
	}
	if len(seen) != 0 {
		t.Errorf("the network was asked: %q", seen)
	}
}

func TestSaveProfileLocalServiceNeedsNoKey(t *testing.T) {
	Init()
	var seen []string
	accts := openWith(t, modelsService(200, `{"data":[{"id":"llama3.2"}]}`, &seen))
	acct, err := accts.SaveProfile(t.Context(), Profile{Name: "Ollama", BaseURL: "http://localhost:11434/v1", DefaultModel: "llama3.2"}, "")
	if err != nil || acct.Profile == "" {
		t.Fatalf("SaveProfile = %+v, %v", acct, err)
	}
	if want := []string{"/v1/models "}; !slices.Equal(seen, want) {
		t.Errorf("the service saw %q, want %q", seen, want)
	}
}
