package orchestrate

import (
	"context"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/tools"
)

// A key that appears in a fetched page is redacted by the scrub the core
// holds, before the model sees the result again and before it is stored.
func TestRun_ScrubCoversAKeyInAFetchedPage(t *testing.T) {
	const key = "sk-live-0123456789abcdef"
	accounts, err := provider.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.Keys.Save("brave-search", key); err != nil {
		t.Fatal(err)
	}
	scrub := accounts.Scrub()

	page := tools.Definition{
		Name: "web.fetch", Description: "a page", Parameters: []byte(`{"type":"object"}`),
		Capability: org.WebFetch, Tier: tools.TierRead,
		Execute: func(context.Context, tools.Call) (tools.Result, error) {
			return tools.Result{Text: "URL: https://example.org\n\nThe docs say to set api_key=" + key + " in the header."}, nil
		},
	}
	f := newFixture(t,
		events(toolCall("c1", "web.fetch", `{}`)),
		events(llm.StreamEvent{Token: "Read it."}, llm.StreamEvent{Done: true}),
	)
	f.svc.extra = []tools.Definition{page}
	f.svc.d.Scrub = func(m agent.Message) agent.Message {
		m.Text = scrub(llm.Message{Content: m.Text}).Content
		return m
	}
	f.send(t, "Read the docs.", ended)

	for _, m := range f.messages(t) {
		if strings.Contains(m.Content, key) {
			t.Errorf("a stored %v message holds the key: %q", m.Kind, m.Content)
		}
	}
	var sawResult bool
	for _, req := range f.script.seen {
		for _, m := range req {
			if strings.Contains(m.Content, key) {
				t.Errorf("the model was sent the key: %q", m.Content)
			}
			sawResult = sawResult || (m.Role == llm.RoleTool && strings.Contains(m.Content, "api_key="))
		}
	}
	if !sawResult {
		t.Error("the model was never sent the page's result")
	}
}
