package deliverable

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuin/goldmark/ast"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixtures(t *testing.T) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	m := map[string][]byte{}
	for _, p := range paths {
		m[filepath.Base(p)] = fixture(t, filepath.Base(p))
	}
	return m
}

func TestAuthored(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"template": {`{"version":1,"kind":"report","title":"T","template":"report","sections":[]}`, "the template is set from the tool's template argument; leave it out of the spec"},
		"as_of":    {doc(`{"kind":"metric","label":"L","measure":"team.size","as_of":"2026-10-20T09:30Z"}`), "section 1, widget 1: as_of is filled in when Hearth writes a bound value; leave it out"},
	} {
		t.Run(name, func(t *testing.T) {
			s, err := Parse([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if err := Authored(s); err == nil || !errors.Is(err, ErrInvalid) || err.Error() != tc.want {
				t.Errorf("Authored = %v; want %q", err, tc.want)
			}
		})
	}
	s, _ := Parse(fixture(t, "report.json"))
	if err := Authored(s); err != nil {
		t.Errorf("a plain spec was refused: %v", err)
	}
}

func TestAccepted(t *testing.T) {
	for name, b := range fixtures(t) {
		s, err := Parse(b)
		if err == nil {
			err = Validate(s)
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEveryKindIsInTheFixture(t *testing.T) {
	s, err := Parse(fixture(t, "kitchen-sink.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[Kind]bool{}
	for _, sec := range s.Sections {
		for _, w := range sec.Widgets {
			seen[w.Kind] = true
		}
	}
	for _, k := range Kinds() {
		if !seen[k] {
			t.Errorf("the kitchen sink has no %s", k)
		}
	}
	if len(Kinds()) != 10 {
		t.Errorf("the catalogue has %d kinds; it is closed at ten", len(Kinds()))
	}
}

func TestCanonical(t *testing.T) {
	for name, b := range fixtures(t) {
		s, err := Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		once, err := Canonical(s)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Parse(once)
		if err != nil {
			t.Fatalf("%s: canonical form does not parse: %v", name, err)
		}
		if err := Validate(again); err != nil {
			t.Fatalf("%s: canonical form is refused: %v", name, err)
		}
		twice, _ := Canonical(again)
		if !bytes.Equal(once, twice) {
			t.Errorf("%s: Canonical is not idempotent:\n%s\n%s", name, once, twice)
		}
		if !bytes.HasSuffix(once, []byte("}\n")) || bytes.Contains(once, []byte("\r")) {
			t.Errorf("%s: canonical form must end in one newline with no carriage return", name)
		}
	}
}

func TestCanonicalIgnoresFormatting(t *testing.T) {
	a := `{"version":1,"kind":"dashboard","title":"A & B <c>","sections":[{"widgets":[
	  {"kind":"metric","value":1.0,"label":"L","format":"number","span":1},
	  {"kind":"list","items":[{"text":"x"}],"style":"bullet","title":"T"},
	  {"kind":"table","columns":[{"header":"h","align":"left"}],"rows":[[1e2]]}]}]}`
	b := `{"sections":[{"columns":2,"widgets":[{"label":"L","kind":"metric","value":1},
	  {"title":"T","kind":"list","items":[{"text":"x"}]},
	  {"kind":"table","rows":[[100.00]],"columns":[{"header":"h"}]}]}],"title":"A & B <c>","kind":"dashboard","version":1}`
	got := make([][]byte, 2)
	for i, in := range []string{a, b} {
		s, err := Parse([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		if got[i], err = Canonical(s); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(got[0], got[1]) {
		t.Errorf("two spellings of one spec differ:\n%s\n%s", got[0], got[1])
	}
	if !bytes.Contains(got[0], []byte(`"title": "A & B <c>"`)) {
		t.Errorf("html characters were escaped:\n%s", got[0])
	}
	if i, j := bytes.Index(got[0], []byte(`"kind"`)), bytes.Index(got[0], []byte(`"sections"`)); i > j {
		t.Errorf("keys are not sorted:\n%s", got[0])
	}
}

func TestCanonicalNumbers(t *testing.T) {
	s, err := Parse([]byte(doc(`{"kind":"metric","label":"L","value":-0.0,"delta":{"value":2.50}}`)))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := Canonical(s)
	for _, want := range []string{`"value": 0`, `"value": 2.5`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("canonical form lacks %s:\n%s", want, out)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, tc := range []struct {
		v        float64
		format   string
		decimals int
		want     string
	}{
		{1234567.891, "number", -1, "1,234,567.891"},
		{1234567.5, "number", 0, "1,234,568"},
		{12, "number", -1, "12"},
		{12.5, "number", 2, "12.50"},
		{0.0004, "number", -1, "0"},
		{-0.4, "number", 0, "0"},
		{2.5, "integer", -1, "3"},
		{-2.5, "integer", -1, "-3"},
		{12.4, "currency", -1, "$12.40"},
		{-1234.5, "currency", -1, "-$1,234.50"},
		{12.34, "percent", -1, "12.3%"},
		{100, "percent", 0, "100%"},
		{999, "number", -1, "999"},
		{1000, "number", -1, "1,000"},
	} {
		if got := Format(tc.v, tc.format, tc.decimals); got != tc.want {
			t.Errorf("Format(%v, %q, %d) = %q; want %q", tc.v, tc.format, tc.decimals, got, tc.want)
		}
	}
}

func TestDemoteHeading(t *testing.T) {
	for level, want := range map[int]int{1: 2, 2: 2, 3: 3, 6: 6} {
		if got := DemoteHeading(level); got != want {
			t.Errorf("DemoteHeading(%d) = %d; want %d", level, got, want)
		}
	}
}

func TestParseMarkdownKeepsHTMLAsNodes(t *testing.T) {
	src := []byte("# Title\n\n<div>x</div>\n")
	var heading, html int
	_ = ast.Walk(ParseMarkdown(src), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Heading:
			heading++
		case *ast.HTMLBlock:
			html++
		}
		return ast.WalkContinue, nil
	})
	if heading != 1 || html != 1 {
		t.Errorf("found %d headings and %d HTML blocks; want one of each", heading, html)
	}
}

func TestAllowedMarkdown(t *testing.T) {
	for _, src := range []string{
		"# Title\n\n## Sub\n\ntext **bold** *it* ~~gone~~ `code`\n\n```\nfenced <b>\n```\n\n    indented <b>\n",
		"- [x] done\n- [ ] open\n\n1. one\n2. two\n\n> quote\n\n---\n",
		"| a | b |\n|---|---|\n| 1 | 2 |\n",
		"[web](https://example.com/a?b=c) [plain](http://example.com) [mail](mailto:a@example.com) <https://example.com> <a@example.com>",
		"a < b and 3 > 2, & so on",
	} {
		if err := markdown("text", src, 20000); err != nil {
			t.Errorf("%q was refused: %v", src, err)
		}
	}
}

// FuzzParse feeds Parse, Validate and Canonical arbitrary bytes. None may
// panic, and what validates must write a canonical form that parses and
// validates again to the same bytes.
func FuzzParse(f *testing.F) {
	paths, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	for _, tc := range refusals {
		f.Add([]byte(tc.in))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := Parse(data)
		if err != nil || Validate(s) != nil {
			return
		}
		once, err := Canonical(s)
		if err != nil {
			t.Fatalf("a valid spec has no canonical form: %v", err)
		}
		again, err := Parse(once)
		if err == nil {
			err = Validate(again)
		}
		if err != nil {
			t.Fatalf("the canonical form is refused: %v\n%s", err, once)
		}
		if twice, _ := Canonical(again); !bytes.Equal(once, twice) {
			t.Fatalf("Canonical is not idempotent:\n%s\n%s", once, twice)
		}
	})
}
