package toolset

import (
	"context"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/web"
)

type noSearch struct{}

func (noSearch) Search(context.Context, string, int) ([]web.Hit, error) { return nil, nil }

func names(r *tools.Registry) []string {
	var out []string
	for _, d := range r.All() {
		out = append(out, d.Name)
	}
	return out
}

func TestRegistry(t *testing.T) {
	extra := tools.Definition{Name: "test.extra", Execute: func(context.Context, tools.Call) (tools.Result, error) { return tools.Result{}, nil }}
	cases := []struct {
		name     string
		fetcher  *web.Fetcher
		searcher web.Searcher
		want     []string
	}{
		{"no web", nil, nil, []string{"files.read", "files.write", "files.delete", "ask_owner", "test.extra"}},
		{"search only", nil, noSearch{}, []string{"files.read", "files.write", "files.delete", "ask_owner", "web.search", "test.extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Registry(tc.fetcher, tc.searcher, extra)
			if err != nil {
				t.Fatal(err)
			}
			if got := names(r); !slices.Equal(got, tc.want) {
				t.Fatalf("tools = %v, want %v", got, tc.want)
			}
		})
	}
	if _, err := Registry(nil, nil, extra, extra); err == nil {
		t.Fatal("a tool defined twice was accepted")
	}
}
