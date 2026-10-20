package toolset

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
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

func TestExecute(t *testing.T) {
	wrote := tools.Definition{Name: "test.write", Capability: "test.write", Execute: func(context.Context, tools.Call) (tools.Result, error) {
		return tools.Result{Text: "done", Files: []org.FileRef{{Path: "/f/a.md", Name: "a.md"}}}, nil
	}}
	broke := tools.Definition{Name: "test.panic", Capability: "test.panic", Execute: func(context.Context, tools.Call) (tools.Result, error) {
		panic("boom")
	}}
	reg, err := tools.NewRegistry(wrote, broke)
	if err != nil {
		t.Fatal(err)
	}
	d := tools.Dispatcher{Defs: reg, Grants: []org.Grant{{Capability: "test.write", Mode: org.ModeAllowed}, {Capability: "test.panic", Mode: org.ModeAllowed}}}
	var crashed any
	exec := Execute(d, func(p any) { crashed = p })

	res, err := exec(context.Background(), "test.write", json.RawMessage(`{}`))
	if err != nil || res.Text != "done" || !slices.Equal(res.Files, []string{"/f/a.md"}) {
		t.Fatalf("Execute = %+v, %v", res, err)
	}
	if _, err := exec(context.Background(), "test.panic", json.RawMessage(`{}`)); err == nil || err.Error() != "test.panic stopped unexpectedly" || crashed != "boom" {
		t.Fatalf("Execute of a panicking tool = %v, crashed %v", err, crashed)
	}
	if _, err := exec(context.Background(), "nope", json.RawMessage(`{}`)); !errors.Is(err, tools.ErrRefused) {
		t.Fatalf("Execute of an unknown tool = %v, want the refusal", err)
	}
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
