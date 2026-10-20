package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// botShaped is what a bot sends for a template: the sample's sections with
// the headings and columns it must leave out, and no template id.
func botShaped(t Template) deliverable.Spec {
	s := t.Sample
	s.Template = ""
	s.Sections = append([]deliverable.Section(nil), s.Sections...)
	for i := range s.Sections {
		s.Sections[i].Heading, s.Sections[i].Columns = "", 0
		s.Sections[i].Widgets = append([]deliverable.Widget(nil), s.Sections[i].Widgets...)
	}
	return s
}

func TestShippedFive(t *testing.T) {
	l := Load("")
	if len(l.Problems) != 0 {
		t.Fatalf("shipped templates with problems: %+v", l.Problems)
	}
	var ids []string
	for _, tpl := range l.Templates {
		ids = append(ids, tpl.ID)
		if !tpl.Shipped {
			t.Errorf("%s is not marked shipped", tpl.ID)
		}
	}
	if got, want := strings.Join(ids, " "), "comparison dashboard one-page-brief report weekly-digest"; got != want {
		t.Errorf("shipped ids = %q; want %q", got, want)
	}
}

func TestShippedParseValidateAndRenderTheirSample(t *testing.T) {
	for _, tpl := range Shipped() {
		t.Run(tpl.ID, func(t *testing.T) {
			if err := deliverable.Validate(tpl.Sample); err != nil {
				t.Fatalf("sample is refused: %v", err)
			}
			out, err := Apply(tpl, botShaped(tpl))
			if err != nil {
				t.Fatalf("Apply of itself: %v", err)
			}
			want, _ := deliverable.Canonical(tpl.Sample)
			got, _ := deliverable.Canonical(out)
			if string(got) != string(want) {
				t.Errorf("Apply of the sample differs from the sample:\n%s\n%s", got, want)
			}
			if out.Template != tpl.ID {
				t.Errorf("Template = %q; want %q", out.Template, tpl.ID)
			}
		})
	}
}

func TestShippedKindsAndSections(t *testing.T) {
	want := map[string]struct {
		kind     string
		headings string
	}{
		"report":         {"report", "Summary|Findings|Recommendation"},
		"one-page-brief": {"report", "The ask|Context|Options|Recommendation"},
		"comparison":     {"report", "What is compared|Side by side|Where they differ|Verdict"},
		"weekly-digest":  {"report", "This week|The numbers|Trend|Next week"},
		"dashboard":      {"dashboard", "Status|Trends|Open items|Notes"},
	}
	for _, tpl := range Shipped() {
		var hs []string
		for _, s := range tpl.Sample.Sections {
			hs = append(hs, s.Heading)
		}
		w := want[tpl.ID]
		if tpl.Sample.Kind != w.kind || strings.Join(hs, "|") != w.headings {
			t.Errorf("%s = %s %v; want %s %s", tpl.ID, tpl.Sample.Kind, hs, w.kind, w.headings)
		}
	}
}

func shippedByID(t *testing.T, id string) Template {
	t.Helper()
	for _, tpl := range Shipped() {
		if tpl.ID == id {
			return tpl
		}
	}
	t.Fatalf("no shipped template %s", id)
	return Template{}
}

func TestApplyRefusals(t *testing.T) {
	digest := shippedByID(t, "weekly-digest")
	mutate := func(f func(*deliverable.Spec)) deliverable.Spec {
		s := botShaped(digest)
		f(&s)
		return s
	}
	for _, tc := range []struct {
		name string
		spec deliverable.Spec
		want string
	}{
		{"template set by the bot", mutate(func(s *deliverable.Spec) { s.Template = "report" }), "the template is set from the tool's template argument; leave it out of the spec"},
		{"as_of set by the bot", mutate(func(s *deliverable.Spec) { s.Sections[1].Widgets[0].AsOf = "2026-10-20T09:30Z" }), "as_of is filled in when Hearth writes a bound value"},
		{"another kind", mutate(func(s *deliverable.Spec) { s.Kind = "dashboard" }), `the template weekly-digest makes a report; the spec says "dashboard"`},
		{"too few sections", mutate(func(s *deliverable.Spec) { s.Sections = s.Sections[:3] }), "weekly-digest has 4 sections; the spec has 3; give one for each, in order"},
		{"too many sections", mutate(func(s *deliverable.Spec) { s.Sections = append(s.Sections, s.Sections[0]) }), "weekly-digest has 4 sections; the spec has 5"},
		{"a heading", mutate(func(s *deliverable.Spec) { s.Sections[1].Heading = "Mine" }), "section 2's heading comes from the template; leave it out"},
		{"columns", mutate(func(s *deliverable.Spec) { s.Sections[2].Columns = 2 }), "section 3's columns come from the template; leave them out"},
		{"a widget the section does not take", mutate(func(s *deliverable.Spec) { s.Sections[1].Widgets[0].Kind = deliverable.Table }), `section "The numbers" of weekly-digest takes metric, progress or light; it was given a table`},
		{"one kind allowed", mutate(func(s *deliverable.Spec) { s.Kind = "report" }), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Apply(digest, tc.spec)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || !errors.Is(err, deliverable.ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Apply = %v; want a refusal saying %q", err, tc.want)
			}
		})
	}
}

func TestApplyKeepsTheOrdinaryRules(t *testing.T) {
	digest := shippedByID(t, "weekly-digest")
	s := botShaped(digest)
	s.Sections[0].Widgets = []deliverable.Widget{{Kind: deliverable.Text, Markdown: "<b>hi</b>"}}
	if _, err := Apply(digest, s); err == nil || !strings.Contains(err.Error(), "has HTML in it") {
		t.Errorf("Apply = %v; want the Markdown refusal", err)
	}
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "Templates")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func owned(id string) string {
	return `{"version":1,"kind":"report","title":"Mine","template":{"id":"` + id + `","name":"Mine","when":"When I say so."},
	"sections":[{"heading":"One","columns":1,"allow":["text"],"hint":"Say it.","widgets":[{"kind":"text","markdown":"x"}]}]}`
}

func TestLoadOwned(t *testing.T) {
	root := t.TempDir()
	write(t, root, "zeta.deliverable.json", owned("zeta"))
	write(t, root, "alpha.deliverable.json", owned("alpha"))
	write(t, root, "notes.txt", "not a template")
	l := Load(root)
	if len(l.Problems) != 0 {
		t.Fatalf("problems: %+v", l.Problems)
	}
	var ids []string
	for _, tpl := range l.Templates {
		ids = append(ids, tpl.ID)
	}
	if got, want := strings.Join(ids, " "), "comparison dashboard one-page-brief report weekly-digest alpha zeta"; got != want {
		t.Errorf("ids = %q; want %q", got, want)
	}
	if l.Templates[5].Shipped {
		t.Error("an owner's template is marked shipped")
	}
}

func TestLoadProblems(t *testing.T) {
	root := t.TempDir()
	write(t, root, "bad.deliverable.json", `{"version":1`)
	write(t, root, "report.deliverable.json", owned("report"))
	write(t, root, "named.deliverable.json", owned("other"))
	write(t, root, "huge.deliverable.json", `{"x":"`+strings.Repeat("a", deliverable.MaxBytes)+`"}`)
	write(t, root, "noallow.deliverable.json", strings.Replace(owned("noallow"), `"allow":["text"]`, `"allow":[]`, 1))
	write(t, root, "wrongkind.deliverable.json", strings.Replace(owned("wrongkind"), `"allow":["text"]`, `"allow":["table"]`, 1))
	write(t, root, "badid.deliverable.json", owned("Bad Id"))
	write(t, root, "ok.deliverable.json", owned("ok"))
	if err := os.Mkdir(filepath.Join(root, "Templates", "dir.deliverable.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "Templates", "ok.deliverable.json"), filepath.Join(root, "Templates", "link.deliverable.json")); err != nil {
		t.Fatal(err)
	}
	l := Load(root)
	got := map[string]string{}
	for _, p := range l.Problems {
		got[p.File] = p.Reason
	}
	for file, want := range map[string]string{
		"bad.deliverable.json":       "the template is not valid JSON",
		"report.deliverable.json":    "report is a shipped template; pick another id",
		"named.deliverable.json":     "the file is named named but the template id is other; make them the same",
		"huge.deliverable.json":      "the limit is 256 KB",
		"noallow.deliverable.json":   "section 1 allows 0 kinds; give 1 to 10",
		"wrongkind.deliverable.json": "the example in section 1 is a text, which its section does not allow",
		"badid.deliverable.json":     "the template id",
		"dir.deliverable.json":       "a template is a plain file",
		"link.deliverable.json":      "a template is a plain file",
	} {
		if !strings.Contains(got[file], want) {
			t.Errorf("%s: problem = %q; want it to say %q", file, got[file], want)
		}
	}
	if len(l.Problems) != 9 {
		t.Errorf("%d problems; want 9: %+v", len(l.Problems), l.Problems)
	}
	if last := l.Templates[len(l.Templates)-1]; last.ID != "ok" || len(l.Templates) != 6 {
		t.Errorf("templates = %d, last %s; want the shipped five and ok", len(l.Templates), last.ID)
	}
}

func TestLoadReadsOneFolderOnly(t *testing.T) {
	root := t.TempDir()
	write(t, root, "sub/deep.deliverable.json", owned("deep"))
	if err := os.WriteFile(filepath.Join(root, "top.deliverable.json"), []byte(owned("top")), 0o644); err != nil {
		t.Fatal(err)
	}
	if l := Load(root); len(l.Templates) != 5 || len(l.Problems) != 0 {
		t.Errorf("loaded %d templates, %d problems; want only the shipped five", len(l.Templates), len(l.Problems))
	}
}

func TestLoadCapsTheFolder(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 53; i++ {
		id := fmt.Sprintf("t%02d", i)
		write(t, root, id+".deliverable.json", owned(id))
	}
	l := Load(root)
	if len(l.Templates) != 5+50 || len(l.Problems) != 1 || !strings.Contains(l.Problems[0].Reason, "more than 50 templates") {
		t.Errorf("%d templates, problems %+v; want 55 and one about the cap", len(l.Templates), l.Problems)
	}
}

func TestLoadWithoutAFolder(t *testing.T) {
	if l := Load(t.TempDir()); len(l.Templates) != 5 || len(l.Problems) != 0 {
		t.Errorf("an absent Templates folder gave %d templates, %d problems", len(l.Templates), len(l.Problems))
	}
}

func TestParseRefusals(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"unknown template field":  {strings.Replace(owned("x"), `"when"`, `"colour":"red","when"`, 1), "the template of the template should be"},
		"unknown top field":       {strings.Replace(owned("x"), `"kind":"report"`, `"kind":"report","colour":1`, 1), `the field "colour" is not one a deliverable takes`},
		"no sections":             {`{"version":1,"kind":"report","title":"T","template":{"id":"x","name":"N","when":"W"}}`, "the spec has 0 sections"},
		"a hint that is too long": {strings.Replace(owned("x"), `"Say it."`, `"`+strings.Repeat("h", 201)+`"`, 1), "the hint of section 1"},
		"no name":                 {strings.Replace(owned("x"), `"name":"Mine"`, `"name":""`, 1), "the template name is 1 to 60 characters"},
		"unknown allowed kind":    {strings.Replace(owned("x"), `["text"]`, `["text","gauge"]`, 1), `section 1 allows "gauge"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.in))
			if err == nil || !errors.Is(err, deliverable.ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Parse = %v; want a refusal saying %q", err, tc.want)
			}
		})
	}
}
