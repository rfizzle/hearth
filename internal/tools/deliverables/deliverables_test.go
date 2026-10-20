package deliverables

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/deliverable/template"
	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

var readAt = time.Date(2026, 10, 21, 9, 30, 45, 0, time.UTC)

// figures is a fake measure reader: aliases resolve as the registry resolves
// them, and each read id answers from values.
type figures struct {
	values map[string]measure.Value
	reads  int
}

func (f *figures) Resolve(id measure.ID, s measure.Scope) (measure.ID, error) {
	return measure.Deps{Now: func() time.Time { return readAt }}.Resolve(id, s)
}

func (f *figures) Read(_ context.Context, ids []measure.ID) ([]measure.Value, error) {
	f.reads++
	out := make([]measure.Value, len(ids))
	for i, id := range ids {
		v, ok := f.values[id.String()]
		if !ok {
			return nil, errors.New("no such figure in the fake")
		}
		v.ID, v.AsOf = id.String(), readAt
		out[i] = v
	}
	return out, nil
}

// rig is a bot's folder, Dana, with write granted in mode, the tool behind a
// dispatcher, and a folder beside it that is not granted.
type rig struct {
	folder, outside string
	d               tools.Dispatcher
	f               *figures
}

func newRig(t *testing.T, mode org.Mode, projectID string) rig {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := rig{folder: filepath.Join(base, "Dana"), outside: filepath.Join(base, "elsewhere")}
	for _, dir := range []string{r.folder, r.outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r.f = &figures{values: map[string]measure.Value{
		"project.tasks/p1":          {Kind: measure.Progress, Done: 3, Total: 7},
		"project.tasks/gone":        {Kind: measure.Progress, Missing: measure.NoSuchProject},
		"project.light/p1":          {Kind: measure.Light, Tone: measure.ToneAttention, Word: "Waiting on you"},
		"usage.cost/bot1/2026-10":   {Kind: measure.Number, Number: 12_340_000, Unit: "usd_micros"},
		"usage.budget/bot1/2026-10": {Kind: measure.Progress, Missing: "No budget set"},
		"usage.tokens/org/2026-10":  {Kind: measure.Number, Number: 4000, Unit: "tokens"},
		"handbook.meter/everyone":   {Kind: measure.Progress, Done: 900, Total: 800},
	}}
	def := Definition(Options{Templates: template.Shipped, Measures: r.f, BotID: "bot1", ProjectID: projectID})
	reg, err := tools.NewRegistry(def)
	if err != nil {
		t.Fatal(err)
	}
	r.d = tools.Dispatcher{Defs: reg, Root: r.folder, Grants: []org.Grant{{Capability: org.FilesWrite, Mode: mode, Folder: r.folder}}}
	return r
}

func (r rig) call(t *testing.T, a map[string]any) (tools.Result, error) {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return r.d.Execute(context.Background(), name, raw)
}

// entries is every name in the bot's folder, sorted.
func (r rig) entries(t *testing.T) []string {
	t.Helper()
	es, err := os.ReadDir(r.folder)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

// digest is a weekly digest's four sections, filled: the numbers section
// binds a progress to this project's tasks and gives a value of its own.
func digest() map[string]any {
	return map[string]any{
		"version": 1, "kind": "report", "title": "Office this week",
		"sections": []any{
			map[string]any{"widgets": []any{map[string]any{"kind": "text", "markdown": "We shipped the **first** issue."}}},
			map[string]any{"widgets": []any{
				map[string]any{"kind": "metric", "label": "Spent", "value": 999, "measure": "usage.cost/me/current"},
				map[string]any{"kind": "progress", "label": "Tasks done", "done": 99, "total": 100, "measure": "project.tasks/this"},
				map[string]any{"kind": "light", "label": "Launch", "measure": "project.light/this"},
			}},
			map[string]any{"widgets": []any{map[string]any{"kind": "bar", "title": "Cost per week", "categories": []string{"W41", "W42"},
				"series": []any{map[string]any{"name": "Spend", "values": []float64{3, 4}}}}}},
			map[string]any{"widgets": []any{map[string]any{"kind": "list", "style": "check", "items": []any{map[string]any{"text": "Send it"}}}}},
		},
	}
}

func TestWrite_ADigest(t *testing.T) {
	r := newRig(t, org.ModeAllowed, "p1")
	res, err := r.call(t, map[string]any{"path": "weekly-digest", "template": "weekly-digest", "spec": digest()})
	if err != nil {
		t.Fatalf("deliverables.write: %v", err)
	}
	specPath, pagePath := filepath.Join(r.folder, "weekly-digest.deliverable.json"), filepath.Join(r.folder, "weekly-digest.html")
	want := []org.FileRef{{Path: specPath, Name: "weekly-digest.deliverable.json"}, {Path: pagePath, Name: "weekly-digest.html"}}
	if !slices.Equal(res.Files, want) {
		t.Errorf("Files = %+v, want %+v", res.Files, want)
	}
	for _, w := range []string{"Wrote weekly-digest.deliverable.json and weekly-digest.html (a weekly digest, 6 widgets, 3 bound to live measures).", "The PDF is not made yet"} {
		if !strings.Contains(res.Text, w) {
			t.Errorf("result %q is missing %q", res.Text, w)
		}
	}
	if strings.Contains(res.Text, r.folder) {
		t.Errorf("result %q names the absolute folder", res.Text)
	}
	if got := r.entries(t); !slices.Equal(got, []string{"weekly-digest.deliverable.json", "weekly-digest.html"}) {
		t.Errorf("the folder holds %v", got)
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := deliverable.Parse(data)
	if err != nil {
		t.Fatalf("the written spec does not parse: %v", err)
	}
	if canon, _ := deliverable.Canonical(spec); !bytes.Equal(canon, data) {
		t.Error("the written spec is not canonical")
	}
	if spec.Template != "weekly-digest" || spec.Sections[1].Heading != "The numbers" || spec.Sections[1].Columns != 3 {
		t.Errorf("the template was not applied: %q, %+v", spec.Template, spec.Sections[1])
	}
	nums := spec.Sections[1].Widgets
	if m := nums[0]; m.Measure != "usage.cost/bot1/2026-10" || m.Value == nil || *m.Value != 12.34 || m.Format != "currency" {
		t.Errorf("the metric = %+v, value %v", m, m.Value)
	}
	if p := nums[1]; p.Measure != "project.tasks/p1" || *p.Done != 3 || *p.Total != 7 || p.AsOf != "2026-10-21T09:30Z" {
		t.Errorf("the bound progress = %+v (done %v, total %v); want the reader's 3 of 7 as of 2026-10-21T09:30Z", p, *p.Done, *p.Total)
	}
	if l := nums[2]; l.Tone != "attention" || l.Word != "Waiting on you" || l.Measure != "project.light/p1" {
		t.Errorf("the light = %+v", l)
	}
	page, err := os.ReadFile(pagePath)
	if err != nil || !bytes.Contains(page, []byte("Office this week")) || bytes.Contains(page, []byte("<script")) {
		t.Errorf("the page: %v, %d bytes", err, len(page))
	}
	// Again, over the first: the same bytes, the files replaced.
	if _, err := r.call(t, map[string]any{"path": "weekly-digest.deliverable.json", "template": "weekly-digest", "spec": digest()}); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(specPath); !bytes.Equal(again, data) {
		t.Error("writing the same spec twice gave different bytes")
	}
}

func TestWrite_FillsWhatTheOfficeKeeps(t *testing.T) {
	r := newRig(t, org.ModeAllowed, "")
	spec := map[string]any{"version": 1, "kind": "dashboard", "title": "Budget", "sections": []any{map[string]any{"widgets": []any{
		map[string]any{"kind": "progress", "label": "Budget", "done": 1, "total": 2, "measure": "usage.budget/me/current"},
		map[string]any{"kind": "progress", "label": "Handbook", "measure": "handbook.meter/org"},
		map[string]any{"kind": "metric", "label": "Tokens", "value": 1, "format": "currency", "measure": "usage.tokens/org/current"},
	}}}}
	if _, err := r.call(t, map[string]any{"path": "budget", "spec": spec}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(r.folder, "budget.deliverable.json"))
	got, err := deliverable.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	ws := got.Sections[0].Widgets
	if b := ws[0]; b.Done != nil || b.Total != nil || b.Caption != "No budget set" || b.AsOf == "" {
		t.Errorf("a figure that cannot be known kept a value: %+v", b)
	}
	if h := ws[1]; h.Measure != "handbook.meter/everyone" || *h.Done != 800 || *h.Total != 800 {
		t.Errorf("an overspent meter = %+v", h)
	}
	if m := ws[2]; *m.Value != 4000 || m.Format != "integer" {
		t.Errorf("tokens = %+v", m)
	}
}

func TestValidate_RefusesBeforeTheGate(t *testing.T) {
	text := func(md string) map[string]any {
		return map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{map[string]any{"kind": "text", "markdown": md}}}}}
	}
	bound := func(kind, id string) map[string]any {
		w := map[string]any{"kind": kind, "label": "L", "measure": id}
		return map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{w}}}}
	}
	withTemplate := digest()
	withTemplate["template"] = "weekly-digest"
	asOf := bound("progress", "project.tasks/p1")
	asOf["sections"].([]any)[0].(map[string]any)["widgets"].([]any)[0].(map[string]any)["as_of"] = "2026-10-21T09:30Z"
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"this in a direct conversation", map[string]any{"path": "d", "spec": bound("progress", "project.tasks/this")}, "this one is not in a project"},
		{"a script in a text block", map[string]any{"path": "d", "spec": text("Hi <script>alert(1)</script>")}, "HTML"},
		{"a pie where the template takes text", map[string]any{"path": "d", "template": "weekly-digest", "spec": func() map[string]any {
			s := digest()
			s["sections"].([]any)[0] = map[string]any{"widgets": []any{map[string]any{"kind": "pie", "title": "P", "slices": []any{map[string]any{"label": "a", "value": 1}, map[string]any{"label": "b", "value": 2}}}}}
			return s
		}()}, "takes text or callout; it was given a pie"},
		{"a dashboard for a report template", map[string]any{"path": "d", "template": "weekly-digest", "spec": func() map[string]any {
			s := digest()
			s["kind"] = "dashboard"
			return s
		}()}, "makes a report"},
		{"a template that is not there", map[string]any{"path": "d", "template": "nope", "spec": text("x")}, `no template "nope"`},
		{"a template set in the spec", map[string]any{"path": "d", "spec": withTemplate}, "the template is set from the tool's template argument"},
		{"as_of from the bot", map[string]any{"path": "d", "spec": asOf}, "as_of is filled in when Hearth writes"},
		{"a measure of the wrong kind", map[string]any{"path": "d", "spec": bound("metric", "project.tasks/p1")}, "project.tasks reads a progress, and a metric widget shows a number"},
		{"a delta beside a bound metric", map[string]any{"path": "d", "spec": func() map[string]any {
			s := bound("metric", "usage.cost/me/current")
			s["sections"].([]any)[0].(map[string]any)["widgets"].([]any)[0].(map[string]any)["delta"] = map[string]any{"value": -35, "good": true}
			return s
		}()}, "a bound metric takes no delta"},
		{"an unknown family", map[string]any{"path": "d", "spec": bound("metric", "weather.today")}, "is not a measure"},
		{"an extension on the path", map[string]any{"path": "d.html", "spec": text("x")}, "leave out .html"},
		{"a folder for a path", map[string]any{"path": "reports/", "spec": text("x")}, "give a name, not a folder"},
		{"no spec", map[string]any{"path": "d"}, "spec is required"},
		{"an unknown argument", map[string]any{"path": "d", "spec": text("x"), "title": "x"}, "the arguments are a JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, org.ModeAsk, "")
			raw, _ := json.Marshal(tc.args)
			_, err := r.d.Check(name, raw)
			if err == nil || !errors.Is(err, tools.ErrRefused) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Check: %v, want a refusal with %q", err, tc.want)
			}
			if _, err := r.d.ExecuteApproved(context.Background(), name, raw); err == nil {
				t.Fatal("an approved call ran what Check refused")
			}
			if got := r.entries(t); len(got) != 0 || r.f.reads != 0 {
				t.Errorf("something was written (%v) or read (%d)", got, r.f.reads)
			}
		})
	}
}

func TestExecute_RefusesAGoneSubject(t *testing.T) {
	r := newRig(t, org.ModeAllowed, "")
	spec := map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{
		map[string]any{"kind": "progress", "label": "L", "measure": "project.tasks/gone"}}}}}
	_, err := r.call(t, map[string]any{"path": "d", "spec": spec})
	if err == nil || !strings.Contains(err.Error(), "project.tasks/gone: no such project") || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v", err)
	}
	if got := r.entries(t); len(got) != 0 {
		t.Errorf("the folder holds %v", got)
	}
}

func TestDescribe_TheCardUnderAsk(t *testing.T) {
	r := newRig(t, org.ModeAsk, "p1")
	if err := os.MkdirAll(filepath.Join(r.folder, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"path": "reports/../weekly-digest", "template": "weekly-digest", "spec": digest()})
	c, err := r.d.Check(name, raw)
	if err != nil {
		t.Fatal(err)
	}
	want := `Write a weekly digest "Office this week" (6 widgets) as weekly-digest.deliverable.json and .html in Dana's folder`
	if c.Verdict != tools.Gated || c.Describe() != want {
		t.Errorf("verdict %v, card %q; want %q", c.Verdict, c.Describe(), want)
	}
	if c.Def.Capability != org.FilesWrite || !c.Def.Standing() {
		t.Errorf("checked as %s, standing %v", c.Def.Capability, c.Def.Standing())
	}
	if _, err := r.d.Execute(context.Background(), name, raw); err == nil {
		t.Error("a call that waits for the owner ran")
	}
	if got := r.entries(t); !slices.Equal(got, []string{"reports"}) {
		t.Errorf("a gated call wrote %v", got)
	}
}

func TestWrite_StaysInsideTheFolder(t *testing.T) {
	spec := map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{map[string]any{"kind": "text", "markdown": "x"}}}}}
	r := newRig(t, org.ModeAllowed, "")
	res, err := r.call(t, map[string]any{"path": "../Dana/drafts/d", "spec": spec})
	if err != nil {
		t.Fatalf("a path back into the folder: %v", err)
	}
	if res.Files[0].Path != filepath.Join(r.folder, "drafts", "d.deliverable.json") {
		t.Errorf("written at %s", res.Files[0].Path)
	}
	if !strings.HasPrefix(res.Text, "Wrote drafts/d.deliverable.json and drafts/d.html ") {
		t.Errorf("text %q", res.Text)
	}
	if _, err := r.call(t, map[string]any{"path": "../elsewhere/d", "spec": spec}); err == nil || !strings.Contains(err.Error(), "outside the folders") {
		t.Errorf("a path out of the folder: %v", err)
	}
	if _, err := r.call(t, map[string]any{"path": ".", "spec": spec}); err == nil || !strings.Contains(err.Error(), "give a name, not a folder") {
		t.Errorf("the folder itself: %v", err)
	}
	if es, _ := os.ReadDir(r.outside); len(es) != 0 {
		t.Errorf("the folder outside holds %d entries", len(es))
	}
}

func TestWrite_RefusesALinkAtAnOutput(t *testing.T) {
	spec := map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{map[string]any{"kind": "text", "markdown": "x"}}}}}
	r := newRig(t, org.ModeAllowed, "")
	if err := os.Symlink(filepath.Join(r.outside, "d.html"), filepath.Join(r.folder, "d.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(r.folder, "e.deliverable.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := r.call(t, map[string]any{"path": "d", "spec": spec})
	if err == nil || !strings.Contains(err.Error(), "d.html") || !strings.Contains(err.Error(), "Already written: d.deliverable.json") {
		t.Errorf("a link at the page: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.outside, "d.html")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file appeared outside the folder: %v", err)
	}
	_, err = r.call(t, map[string]any{"path": "e", "spec": spec})
	if err == nil || !strings.Contains(err.Error(), "e.deliverable.json is a folder") || strings.Contains(err.Error(), "Already written") {
		t.Errorf("a folder at the spec: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.folder, "e.html")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the page was written after the spec failed: %v", err)
	}
}

func TestWrite_RefusesAReservedOutput(t *testing.T) {
	spec := map[string]any{"version": 1, "kind": "report", "title": "T", "sections": []any{map[string]any{"widgets": []any{map[string]any{"kind": "text", "markdown": "x"}}}}}
	r := newRig(t, org.ModeAllowed, "")
	def := Definition(Options{Reserved: []string{filepath.Join(r.folder, "kept.html")}})
	raw, _ := json.Marshal(map[string]any{"path": "kept", "spec": spec})
	_, err := def.Execute(context.Background(), tools.Call{Args: raw, Path: filepath.Join(r.folder, "kept"), Folder: r.folder})
	if err == nil || !strings.Contains(err.Error(), "kept.html is in a folder Hearth keeps for itself") {
		t.Errorf("err = %v", err)
	}
	if got := r.entries(t); len(got) != 0 {
		t.Errorf("the folder holds %v", got)
	}
}

func TestDescription(t *testing.T) {
	def := Definition(Options{Templates: template.Shipped})
	for _, k := range deliverable.Kinds() {
		if !strings.Contains(def.Description, "\n- "+string(k)+": ") {
			t.Errorf("the description lacks the %s widget", k)
		}
	}
	for _, tpl := range template.Shipped() {
		if !strings.Contains(def.Description, "\n- "+tpl.ID+" (a ") {
			t.Errorf("the description lacks the template %s", tpl.ID)
		}
	}
	for _, s := range measureIDs() {
		id, _, _ := strings.Cut(s, " ")
		if _, err := measure.ParseID(id); err != nil {
			t.Errorf("the description names %s: %v", id, err)
		}
	}
	var schema struct {
		Properties struct {
			Template struct {
				Enum []string `json:"enum"`
			} `json:"template"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(def.Parameters, &schema); err != nil || len(schema.Properties.Template.Enum) != 5 {
		t.Errorf("the template enum = %v, %v", schema.Properties.Template.Enum, err)
	}
	if none := Definition(Options{}); strings.Contains(none.Description, "Pick a template") {
		t.Error("with no templates, the description offers some")
	}
}
