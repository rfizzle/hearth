package html

import (
	"bytes"
	"encoding/json"
	"flag"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/deliverable"
	shipped "github.com/rfizzle/hearth/internal/deliverable/template"
	nethtml "golang.org/x/net/html"
)

// update rewrites the goldens: go test ./internal/render/html -run TestGoldens -update
var update = flag.Bool("update", false, "rewrite the golden files under testdata")

type sample struct {
	name string
	spec deliverable.Spec
}

// pages is every shipped template's sample and the kitchen-sink spec, each
// parsed and validated as the tool would.
func pages(t *testing.T) []sample {
	t.Helper()
	var ps []sample
	for _, tpl := range shipped.Shipped() {
		ps = append(ps, sample{tpl.ID, tpl.Sample})
	}
	ps = append(ps, sample{"kitchen-sink", kitchenSink(t)})
	for _, p := range ps {
		if err := deliverable.Validate(p.spec); err != nil {
			t.Fatalf("%s is not a valid spec: %v", p.name, err)
		}
	}
	if len(ps) != 6 {
		t.Fatalf("want five shipped samples and the kitchen sink, got %d pages", len(ps))
	}
	return ps
}

func kitchenSinkJSON(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "kitchen-sink.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func kitchenSink(t *testing.T) deliverable.Spec {
	t.Helper()
	s, err := deliverable.Parse(kitchenSinkJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func render(t *testing.T, s deliverable.Spec) string {
	t.Helper()
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestGoldens(t *testing.T) {
	for _, p := range pages(t) {
		got := render(t, p.spec)
		path := filepath.Join("testdata", p.name+".html")
		if *update {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden (run with -update): %v", err)
		}
		if got != string(want) {
			t.Errorf("%s differs from its golden; run with -update and read the diff", path)
		}
	}
}

// TestEveryGoldenIsInert reads the committed goldens, not a fresh render, so a
// golden blessed by -update is checked too.
func TestEveryGoldenIsInert(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.html"))
	if err != nil || len(files) != 6 {
		t.Fatalf("want six goldens, got %d (%v)", len(files), err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		assertInert(t, f, string(b), true)
	}
}

// TestKitchenSink holds the string checks on the page: the policy, the dark
// variables, print, every kind drawn, and nothing that runs or fetches.
func TestKitchenSink(t *testing.T) {
	out := render(t, kitchenSink(t))
	for _, want := range []string{
		"<!doctype html>\n<html lang=\"en\">",
		`<meta charset="utf-8">`,
		`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">`,
		`<meta name="generator" content="Hearth">`,
		"<title>Office this week</title>",
		"@media (prefers-color-scheme:dark){:root{",
		"--color-surface-card:#2c2c2e",
		`svg .c6:not([fill="none"]) { fill: var(--color-chart-6); }`,
		"@media print",
		"@page { size: landscape; margin: 16mm; }",
		"break-inside:avoid",
		"Week of 13 October 2026 &amp; what comes next",
		"$12.40", "-$3.10", "12.5%", "&#43;2.5%", "1,234,568", "&#43;1,200", "3.14",
		"As of 2026-10-20 09:30 UTC",
		"3 of 7", `<rect width="42.857" height="8"`,
		"Waiting on you", "On track", "Stopped", "Idle", "Not read yet",
		"Venue &lt;shortlist&gt;", "$1,262.50", "60.3%", ">6<",
		"Send the first issue", "Confirmed Tuesday", "list-decimal",
		"<h2>Heading one reads as two</h2>", "<h6>Six</h6>",
		"<em>emphasis</em>", "<strong>strong</strong>", "<del>struck</del>", "<code>code &lt;b&gt;</code>",
		`<a href="https://example.com/a?b=1&amp;c=2" rel="noopener noreferrer">link</a>`,
		`<a href="https://example.com/auto" rel="noopener noreferrer">https://example.com/auto</a>`,
		`<a href="mailto:dana@example.com" rel="noopener noreferrer">dana@example.com</a>`,
		`<a href="mailto:dana@example.com" rel="noopener noreferrer">Dana</a>`,
		"<br>\n", "5 &lt; 6 &amp; &#34;quotes&#34; &#39;too&#39;",
		`<ol start="3">`, "☑ a done task", "☐ an open task",
		"<blockquote>\n<p>a quote\non two lines</p>\n</blockquote>",
		"<pre><code>fmt.Println(&#34;&lt;b&gt;&#34;)\n</code></pre>", "<pre><code>indented code\n</code></pre>",
		`<th class="text-left">Left</th><th class="text-center">Centre</th><th class="text-right">Right</th>`,
		"<hr>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the kitchen sink lacks %q", want)
		}
	}
	if n := strings.Count(out, "<svg"); n != 3+2 {
		t.Errorf("want three charts and two progress bars, got %d <svg", n)
	}
	for _, kind := range deliverable.Kinds() {
		if !hasKind(kitchenSink(t), kind) {
			t.Errorf("the kitchen sink has no %s", kind)
		}
	}
	assertInert(t, "kitchen sink", out, true)
}

func hasKind(s deliverable.Spec, k deliverable.Kind) bool {
	for _, sec := range s.Sections {
		for _, w := range sec.Widgets {
			if w.Kind == k {
				return true
			}
		}
	}
	return false
}

// assertInert tokenises a page as a browser would and fails on anything that
// runs, fetches or navigates: a script, frame, object, image, form, base or
// link element, an event, src or style attribute, an href that is not http,
// https or mailto, an anchor without rel, or a second <style>.
//
// A page from a valid spec (strict) holds none of their words, not even as
// text; a hostile page may show them as escaped text.
func assertInert(t *testing.T, name, page string, strict bool) {
	t.Helper()
	low := strings.ToLower(page)
	bad := []string{"<script", "<iframe", "<object", "<embed", "<img", "<link", "<base", "<form", "<foreignobject"}
	if strict {
		bad = append(bad, "src=", "url(", "@import", "javascript:", "vbscript:", "data:", "xlink", "target=")
	}
	for _, b := range bad {
		if strings.Contains(low, b) {
			t.Errorf("%s holds %q", name, b)
		}
	}
	z := nethtml.NewTokenizer(strings.NewReader(page))
	styles := 0
	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			break
		}
		if tt != nethtml.StartTagToken && tt != nethtml.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		switch tok.Data {
		case "script", "iframe", "frame", "object", "embed", "img", "image", "link", "base", "form", "input", "button", "video", "audio", "source", "foreignobject":
			t.Errorf("%s holds a <%s> element", name, tok.Data)
		case "style":
			styles++
		case "a":
			if attr(tok, "rel") != "noopener noreferrer" {
				t.Errorf("%s has a link without rel: %v", name, tok)
			}
		case "meta":
			if strings.EqualFold(attr(tok, "http-equiv"), "refresh") {
				t.Errorf("%s has a refresh", name)
			}
		}
		for _, a := range tok.Attr {
			k := strings.ToLower(a.Key)
			switch {
			case strings.HasPrefix(k, "on"), k == "src", k == "srcset", k == "style", k == "formaction", k == "action", k == "xlink:href", k == "background", k == "poster":
				t.Errorf("%s has a %s attribute on <%s>", name, a.Key, tok.Data)
			case k == "href" && !allowedHref.MatchString(a.Val):
				t.Errorf("%s has an href to %q", name, a.Val)
			}
		}
	}
	if styles != 1 {
		t.Errorf("%s has %d <style> elements, want one", name, styles)
	}
	if n := strings.Count(low, "</style"); n != 1 {
		t.Errorf("%s closes a style %d times, want once", name, n)
	}
}

// allowedHref is the link rule written apart from the walker's, so the check
// is not the code it checks.
var allowedHref = regexp.MustCompile(`^(?i:https?://[^/\s\x00-\x1f]|mailto:[^\s\x00-\x1f])[^\s\x00-\x1f\x7f]*$`)

func attr(tok nethtml.Token, key string) string {
	for _, a := range tok.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// TestHostile renders a spec no validation has seen: every plain field and
// the Markdown carry markup, script addresses, an event attribute, a style
// closer and a line separator. The page stays inert and the words stay words.
func TestHostile(t *testing.T) {
	const evil = `<script>alert(1)</script>"><img src=x onerror=alert(1)></style><style>` + " " + `'`
	md := "<script>alert(1)</script>\n\n<div onclick=\"x\">hi</div>\n\n" +
		"[a](javascript:alert(1)) [b](JAVASCRIPT:alert(1)) [c](java\tscript:x) [d](data:text/html,<script>) " +
		"[e](vbscript:x) [f](//evil.example/x) [g](https:) [h](mailto:) <javascript:alert(1)> " +
		"![i](https://example.com/x.png) <img src=x onerror=alert(1)> <span style=\"x\">s</span> " + evil + "\n\n" +
		"```" + `"><script>` + "\n</style>\n```\n"
	f := 1.0
	w := func(k deliverable.Kind) deliverable.Widget {
		return deliverable.Widget{Kind: k, Label: evil, Title: evil, Caption: evil, Unit: evil, Word: evil, Tone: evil,
			Format: evil, Style: evil, AsOf: evil, Measure: evil, Markdown: md, Span: 9, Value: &f, Done: &f, Total: &f}
	}
	table := w(deliverable.Table)
	table.Columns = []deliverable.Column{{Header: evil, Align: evil, Format: evil}}
	table.Rows = [][]any{{evil}, {2.0, evil}}
	table.Footer = []any{evil}
	list := w(deliverable.List)
	list.Items = []deliverable.Item{{Text: evil, Detail: evil, Done: true}}
	line := w(deliverable.Line)
	line.Categories = []string{evil, "b"}
	line.Series = []deliverable.Series{{Name: evil, Values: []float64{1, 2}}}
	pie := w(deliverable.Pie)
	pie.Slices = []deliverable.Slice{{Label: evil, Value: 1}, {Label: "b", Value: 1}}
	s := deliverable.Spec{Version: 1, Kind: evil, Title: evil, Subtitle: evil, Template: evil, Sections: []deliverable.Section{
		{Heading: evil, Columns: 9, Widgets: []deliverable.Widget{w(deliverable.Metric), w(deliverable.Progress), w(deliverable.Light),
			w(deliverable.Callout), w(deliverable.Text), table, list, line, pie, w("gauge")}},
	}}
	s.Sections[0].Widgets[0].Delta = &deliverable.Delta{Value: 1}
	list2 := list
	list2.Style = "check"
	s.Sections[0].Widgets = append(s.Sections[0].Widgets, list2)

	out := render(t, s)
	assertInert(t, "hostile", out, false)
	for _, want := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&lt;/style&gt;", " "} {
		if !strings.Contains(out, want) {
			t.Errorf("the hostile page lacks the escaped %q", want)
		}
	}
	if strings.Contains(out, "evil.example") {
		t.Error("the hostile page holds a refused link's address")
	}
	if strings.Contains(out, "<a ") {
		t.Errorf("the hostile page wrote a link: %s", out[strings.Index(out, "<a "):][:80])
	}
}

func TestWebAddress(t *testing.T) {
	for in, want := range map[string]bool{
		"https://example.com": true, "HTTP://example.com/a?b=1&c=2": true, "mailto:dana@example.com": true,
		"https:": false, "https://": false, "http:///x": false, "mailto:": false, "javascript:alert(1)": false,
		"JavaScript:x": false, " https://example.com": false, "java\tscript:x": false, "https://exa mple.com": false,
		"https://example.com\n": false, "data:text/html,x": false, "file:///etc/passwd": false, "//example.com": false,
		"/relative": false, "https://example.com/\u0085": false, "": false,
	} {
		if got := webAddress(in); got != want {
			t.Errorf("webAddress(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMarkdownDropsWhatIsRefused(t *testing.T) {
	for in, want := range map[string]string{
		"a <b>bold</b> c":                   "<p>a bold c</p>\n",
		"<div>\nblock\n</div>":              "",
		"![alt](https://example.com/x.png)": "<p></p>\n",
		"[x](javascript:alert(1))":          "<p>x</p>\n",
		"[x](https://e.com \"t\")":          `<p><a href="https://e.com" rel="noopener noreferrer">x</a></p>` + "\n",
		"&lt;script&gt; &amp;":              "<p>&lt;script&gt; &amp;</p>\n",
		"a\\\nb":                            "<p>a<br>\nb</p>\n",
	} {
		if got := markdown(in); got != want {
			t.Errorf("markdown(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSameBytes renders a spec twice, and again after its keys are written in
// shuffled orders, parsed, made canonical and parsed back.
func TestSameBytes(t *testing.T) {
	src := kitchenSinkJSON(t)
	want := render(t, kitchenSink(t))
	if again := render(t, kitchenSink(t)); again != want {
		t.Fatal("two renders of one spec differ")
	}
	var tree any
	if err := json.Unmarshal(src, &tree); err != nil {
		t.Fatal(err)
	}
	for seed := range uint64(5) {
		var b bytes.Buffer
		shuffled(&b, tree, rand.New(rand.NewPCG(seed, seed+1)))
		s, err := deliverable.Parse(b.Bytes())
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		canon, err := deliverable.Canonical(s)
		if err != nil {
			t.Fatal(err)
		}
		s, err = deliverable.Parse(canon)
		if err != nil {
			t.Fatal(err)
		}
		if got := render(t, s); got != want {
			t.Errorf("seed %d: the shuffled spec renders differently", seed)
		}
	}
}

// shuffled writes v as JSON with every object's keys in a random order.
func shuffled(b *bytes.Buffer, v any, r *rand.Rand) {
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		r.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			b.Write(kb)
			b.WriteByte(':')
			shuffled(b, v[k], r)
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, x := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			shuffled(b, x, r)
		}
		b.WriteByte(']')
	default:
		vb, _ := json.Marshal(v)
		b.Write(vb)
	}
}

// TestStylesheet checks the inlined CSS: it names the pinned Tailwind version,
// cannot close its own <style>, fetches nothing, and holds the classes the
// Markdown walker writes.
func TestStylesheet(t *testing.T) {
	version, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "tailwind", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(stylesheet, "\n")
	if want := "tailwindcss " + strings.TrimSpace(string(version)) + ","; !strings.Contains(header, want) {
		t.Errorf("the stylesheet's header %q does not name %q", header, want)
	}
	low := strings.ToLower(stylesheet)
	for _, bad := range []string{"</", "<!--", "url(", "@import", "expression("} {
		if strings.Contains(low, bad) {
			t.Errorf("the stylesheet holds %q", bad)
		}
	}
	for _, class := range []string{".text-left{", ".text-center{", ".text-right{"} {
		if !strings.Contains(stylesheet, class) {
			t.Errorf("the stylesheet lacks %s, which the Markdown walker writes", class)
		}
	}
}

// TestClassesAreBuilt fails when a page uses a class the committed stylesheet
// does not define: make tailwind was not run after a template changed. The
// chart's c1 to c6 are styled by the page's own rules.
func TestClassesAreBuilt(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range pages(t) {
		z := nethtml.NewTokenizer(strings.NewReader(render(t, p.spec)))
		for tt := z.Next(); tt != nethtml.ErrorToken; tt = z.Next() {
			for _, c := range strings.Fields(attr(z.Token(), "class")) {
				seen[c] = true
			}
		}
	}
	chartClass := regexp.MustCompile(`^c[1-6]$`)
	for c := range seen {
		if !chartClass.MatchString(c) && !defined(c) {
			t.Errorf("the class %q is not in deliverable.css; run make tailwind", c)
		}
	}
	if len(seen) < 50 {
		t.Errorf("only %d classes seen; the scan is broken", len(seen))
	}
}

// defined holds when the stylesheet has a rule whose selector names c.
func defined(c string) bool {
	sel := "." + cssEscape(c)
	for _, next := range []string{"{", ":", " ", ">", ")", ","} {
		if strings.Contains(stylesheet, sel+next) {
			return true
		}
	}
	return false
}

// cssEscape is a class name as a selector writes it.
func cssEscape(c string) string {
	var b strings.Builder
	for _, r := range c {
		if strings.ContainsRune(`:/[]&>.()=%"'#,*+~!`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// FuzzMarkdownIsInert feeds the walker anything; what it writes stays inert.
// make test runs the seeds; go test -fuzz FuzzMarkdownIsInert runs it longer.
func FuzzMarkdownIsInert(f *testing.F) {
	for _, seed := range []string{"# h\n\n*a* **b** `c` [d](https://e.com)", "<script>x</script>", "[a](javascript:x)", "<img src=x onerror=y>",
		"| a |\n|---|\n| <b> |", "```\n</style>\n```", "&lt;script&gt;", "\\<script\\>", "<https://e.com/\"onmouseover=x>", "[a](<java script:x>)"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		out := "<!doctype html><style></style>" + markdown(src)
		assertInert(t, "markdown", out, false)
	})
}
