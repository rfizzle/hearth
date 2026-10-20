package office

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"
)

const (
	wns       = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	wordMain  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	slideMain = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	sheetMain = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
)

type part struct{ name, body string }

func zipBytes(t *testing.T, parts ...part) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, p := range parts {
		f, err := w.Create(p.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(p.body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func types(partName, typ string) part {
	return part{"[Content_Types].xml", fmt.Sprintf(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="%s" ContentType="%s"/></Types>`, partName, typ)}
}

func open(t *testing.T, lim Limits, parts ...part) *Archive {
	t.Helper()
	b := zipBytes(t, parts...)
	a, err := Open(bytes.NewReader(b), int64(len(b)), lim)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func wordArchive(t *testing.T, body string, extra ...part) *Archive {
	t.Helper()
	parts := append([]part{
		types("/word/document.xml", wordMain),
		{"word/document.xml", `<w:document xmlns:w="` + wns + `"><w:body>` + body + `</w:body></w:document>`},
	}, extra...)
	return open(t, Default, parts...)
}

func run(text string, props string) string {
	return "<w:r>" + props + "<w:t>" + text + "</w:t></w:r>"
}

func TestWordConstructs(t *testing.T) {
	cell := func(s string) string { return "<w:tc><w:p>" + run(s, "") + "</w:p></w:tc>" }
	numbering := part{"word/numbering.xml", `<w:numbering xmlns:w="` + wns + `">` +
		`<w:abstractNum w:abstractNumId="7"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl><w:lvl w:ilvl="1"><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>` +
		`<w:num w:numId="3"><w:abstractNumId w:val="7"/></w:num></w:numbering>`}
	body := `<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr>` + run("Title", "") + `</w:p>` +
		`<w:p>` + run("a ", "") + run("bold", "<w:rPr><w:b/></w:rPr>") + run(" word", "<w:rPr><w:b/></w:rPr>") + run(" off", `<w:rPr><w:b w:val="0"/></w:rPr>`) +
		`<w:r><w:tab/><w:t>x</w:t></w:r><w:r><w:br/><w:t>y</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="3"/></w:numPr></w:pPr>` + run("one", "") + `</w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="3"/></w:numPr></w:pPr>` + run("sub", "") + `</w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="0"/></w:numPr></w:pPr>` + run("not a list", "") + `</w:p>` +
		// A table with a table in a cell: the inner text joins the outer cell.
		`<w:tbl><w:tr>` + cell("H1") + cell("H2") + `</w:tr><w:tr><w:tc><w:p>` + run("outer", "") + `</w:p><w:tbl><w:tr>` + cell("inner") + `</w:tr></w:tbl></w:tc>` + cell("p|q") + `</w:tr></w:tbl>` +
		// A text box: a paragraph inside a paragraph, and the fallback copy of it.
		`<w:p>` + run("before ", "") + `<w:r><mc:AlternateContent xmlns:mc="m"><mc:Choice><w:drawing><w:txbxContent><w:p>` + run("boxed", "") + `</w:p></w:txbxContent></w:drawing></mc:Choice>` +
		`<mc:Fallback><w:pict><w:txbxContent><w:p>` + run("boxed", "") + `</w:p></w:txbxContent></w:pict></mc:Fallback></mc:AlternateContent></w:r>` + run("after", "") + `</w:p>` +
		`<w:p><w:r><w:footnoteReference w:id="1"/></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:i/></w:rPr><w:t> spaced </w:t></w:r></w:p>`
	w, err := wordArchive(t, body, numbering).Word(context.Background(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	want := "## Title\n\n" +
		"a **bold word** off\tx  \ny\n\n" +
		"1. one\n  - sub\n\n" +
		"not a list\n\n" +
		"| H1 | H2 |\n| --- | --- |\n| outer inner | p\\|q |\n\n" +
		"boxed\n\n" +
		"before after\n\n" +
		"_spaced_"
	if w.Text != want {
		t.Errorf("text:\n%q\nwant\n%q", w.Text, want)
	}
	if !w.Images || !w.Footnotes || w.Cut {
		t.Errorf("flags: %+v", w)
	}
}

func TestWordStopsAtTheTextCap(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 50; i++ {
		body.WriteString("<w:p>" + run(strings.Repeat("é", 40), "") + "</w:p>")
	}
	w, err := wordArchive(t, body.String()).Word(context.Background(), 101)
	if err != nil || !w.Cut || len(w.Text) > 101 || strings.ContainsRune(w.Text, '�') {
		t.Errorf("cut at 101: %d bytes, cut %v, %v", len(w.Text), w.Cut, err)
	}
}

func TestDeckReadsInTheDeckOrderAndLeavesNotesOut(t *testing.T) {
	sld := func(lines ...string) string {
		var ps string
		for i, l := range lines {
			ps += fmt.Sprintf(`<a:p><a:pPr lvl="%d"/><a:r><a:t>%s</a:t></a:r><a:br/><a:r><a:t>!</a:t></a:r></a:p>`, i, l)
		}
		return `<p:sld xmlns:a="a" xmlns:p="p">` + ps + `</p:sld>`
	}
	rel := func(id, target string) string {
		return fmt.Sprintf(`<Relationship Id="%s" Target="%s"/>`, id, target)
	}
	a := open(t, Default,
		types("/ppt/presentation.xml", slideMain),
		part{"ppt/presentation.xml", `<p:presentation xmlns:p="p" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>` +
			`<p:sldId id="1" r:id="rB"/><p:sldId id="2" r:id="rA"/><p:sldId id="3" r:id="rC"/><p:sldId id="4" r:id="rD"/><p:sldId id="5" r:id="rE"/></p:sldIdLst></p:presentation>`},
		part{"ppt/_rels/presentation.xml.rels", `<Relationships>` + rel("rA", "slides/a.xml") + rel("rB", "/ppt/slides/b.xml") +
			rel("rC", "../../etc/passwd") + rel("rD", "slides/missing.xml") + rel("rE", "slides/../slides/a.xml") + `</Relationships>`},
		part{"ppt/slides/a.xml", sld("Second", "nested", "deeper")},
		part{"ppt/slides/b.xml", sld("First")},
		part{"ppt/notesSlides/n.xml", sld("secret note")},
		part{"../../evil.xml", "<x/>"},
	)
	d, err := a.Deck(context.Background())
	if err != nil || d.Len() != 3 { // b, a, and a again by a second path; the escaping and missing targets are dropped
		t.Fatalf("deck: %d slides, %v", d.Len(), err)
	}
	first, _ := d.Slide(context.Background(), 1, 1<<20)
	second, _ := d.Slide(context.Background(), 2, 1<<20)
	if first != "First !" || second != "Second !\n- nested !\n  - deeper !" {
		t.Errorf("slides: %q, %q", first, second)
	}
	if _, err := d.Slide(context.Background(), 4, 10); err == nil {
		t.Error("slide past the end read")
	}
	if cut, _ := d.Slide(context.Background(), 2, 4); len(cut) > 4 {
		t.Errorf("slide cap: %q", cut)
	}
}

func TestFormatComesFromTheContentTypes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parts []part
		want  Format
	}{
		{"word", []part{types("/word/document.xml", wordMain), {"word/document.xml", "<x/>"}}, Word},
		{"deck", []part{types("/ppt/presentation.xml", slideMain), {"ppt/presentation.xml", "<x/>"}}, PowerPoint},
		{"book", []part{types("/xl/workbook.xml", sheetMain), {"xl/workbook.xml", "<x/>"}}, Excel},
		{"word type on a deck part", []part{types("/ppt/presentation.xml", wordMain), {"ppt/presentation.xml", "<x/>"}}, ""},
		{"type without its part", []part{types("/word/document.xml", wordMain)}, ""},
		{"neither", []part{types("/other.xml", "text/xml"), {"other.xml", "<x/>"}}, ""},
	} {
		got, err := open(t, Default, tc.parts...).Format(context.Background())
		if err != nil || got != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestOpenRefusals(t *testing.T) {
	if _, err := Open(strings.NewReader("not a zip at all"), 16, Default); !errors.Is(err, ErrDamaged) {
		t.Errorf("not a zip: %v", err)
	}
	b := zipBytes(t, part{"a", "1"}, part{"b", "2"}, part{"c", "3"})
	if _, err := Open(bytes.NewReader(b), int64(len(b)), Limits{Entries: 2}); !errors.Is(err, ErrLimit) {
		t.Errorf("three entries under a cap of two: %v", err)
	}
	b = zipBytes(t, part{"a", "1"}, part{"a", "2"})
	if _, err := Open(bytes.NewReader(b), int64(len(b)), Default); !errors.Is(err, ErrDamaged) {
		t.Errorf("a name twice: %v", err)
	}
}

// lying is a zip whose one part holds size bytes of XML under a header that
// claims claim.
func lying(t *testing.T, size int, claim uint64) []byte {
	t.Helper()
	content := []byte("<r>" + strings.Repeat("x", size) + "</r>")
	var z bytes.Buffer
	fw, _ := flate.NewWriter(&z, flate.BestCompression)
	_, _ = fw.Write(content)
	fw.Close()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.CreateRaw(&zip.FileHeader{Name: "p.xml", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(content),
		CompressedSize64: uint64(z.Len()), UncompressedSize64: claim})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(z.Bytes())
	w.Close()
	return b.Bytes()
}

func walkAll(a *Archive, ctx context.Context) error {
	return a.walk(ctx, "p.xml", func(xml.Token) error { return nil })
}

func TestAPartIsCappedWhateverItsHeaderSays(t *testing.T) {
	lim := Limits{Entries: 10, Entry: 1000, Total: 1 << 20, Depth: 100, Elements: 100}
	for name, tc := range map[string]struct {
		size  int
		claim uint64
		want  error
	}{
		"honest and small":  {size: 500, claim: 507, want: nil},
		"claims a terabyte": {size: 5, claim: 1 << 40, want: ErrLimit},
		// The zip reader itself stops at the claimed size, which it finds
		// short of the data: damage, and never read through.
		"claims small, is large": {size: 50000, claim: 10, want: ErrDamaged},
		"honest and large":       {size: 50000, claim: 50007, want: ErrLimit},
	} {
		b := lying(t, tc.size, tc.claim)
		a, err := Open(bytes.NewReader(b), int64(len(b)), lim)
		if err != nil {
			t.Fatal(err)
		}
		if got := walkAll(a, context.Background()); !errors.Is(got, tc.want) {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

func TestAllThePartsShareTheTotal(t *testing.T) {
	big := "<r>" + strings.Repeat("x", 700) + "</r>"
	b := zipBytes(t, part{"a.xml", big}, part{"b.xml", big})
	a, err := Open(bytes.NewReader(b), int64(len(b)), Limits{Entries: 10, Entry: 1000, Total: 1000, Depth: 10, Elements: 10})
	if err != nil {
		t.Fatal(err)
	}
	nop := func(xml.Token) error { return nil }
	if err := a.walk(context.Background(), "a.xml", nop); err != nil {
		t.Fatalf("first part: %v", err)
	}
	if err := a.walk(context.Background(), "b.xml", nop); !errors.Is(err, ErrLimit) {
		t.Errorf("second part: %v, want ErrLimit", err)
	}
}

func TestXMLDepthAndElementCaps(t *testing.T) {
	lim := Limits{Entries: 10, Entry: 1 << 20, Total: 1 << 20, Depth: 100, Elements: 1000}
	nest := func(n int) []byte {
		return zipBytes(t, part{"p.xml", strings.Repeat("<a>", n) + strings.Repeat("</a>", n)})
	}
	for n, want := range map[int]error{99: nil, 100: nil, 101: ErrLimit, 150: ErrLimit} {
		b := nest(n)
		a, _ := Open(bytes.NewReader(b), int64(len(b)), lim)
		if got := walkAll(a, context.Background()); !errors.Is(got, want) {
			t.Errorf("depth %d: %v, want %v", n, got, want)
		}
	}
	for n, want := range map[int]error{999: nil, 1000: nil, 1001: ErrLimit} {
		b := zipBytes(t, part{"p.xml", "<r>" + strings.Repeat("<a/>", n-1) + "</r>"})
		a, _ := Open(bytes.NewReader(b), int64(len(b)), lim)
		if got := walkAll(a, context.Background()); !errors.Is(got, want) {
			t.Errorf("%d elements: %v, want %v", n, got, want)
		}
	}
}

func TestMalformedXMLIsDamage(t *testing.T) {
	for _, body := range []string{"<a><b></a>", "<a>", "not xml <", `<?xml version="1.0" encoding="UTF-16"?><a/>`, "<a>&nope;</a>"} {
		b := zipBytes(t, part{"p.xml", body})
		a, _ := Open(bytes.NewReader(b), int64(len(b)), Default)
		if err := walkAll(a, context.Background()); !errors.Is(err, ErrDamaged) {
			t.Errorf("%q: %v, want ErrDamaged", body, err)
		}
	}
}

func TestWalkEndsWhenTheContextDoes(t *testing.T) {
	b := zipBytes(t, part{"p.xml", "<r>" + strings.Repeat("<a/>", 5000) + "</r>"})
	a, _ := Open(bytes.NewReader(b), int64(len(b)), Default)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := walkAll(a, ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("before the part: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	n := 0
	err := a.walk(ctx, "p.xml", func(xml.Token) error {
		if n++; n == 100 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || n > 100+2*checkEvery {
		t.Errorf("within the part: %v after %d tokens", err, n)
	}
}

func TestAPanicInAWalkerIsRecovered(t *testing.T) {
	b := zipBytes(t, part{"p.xml", "<a><b/></a>"})
	a, _ := Open(bytes.NewReader(b), int64(len(b)), Default)
	err := a.walk(context.Background(), "p.xml", func(tok xml.Token) error {
		panic("a walker bug")
	})
	if !errors.Is(err, ErrPanic) {
		t.Errorf("err = %v, want ErrPanic", err)
	}
}

func TestCutKeepsWholeCharacters(t *testing.T) {
	for n, want := range map[int]string{0: "", 1: "a", 2: "a", 3: "aé", 9: "aéb"} {
		if got := Cut("aéb", n); got != want {
			t.Errorf("Cut(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestTable(t *testing.T) {
	got := Table([][]string{{"a", "b|c", "d"}, {"1\n2", "x"}, {}})
	want := "| a | b\\|c | d |\n| --- | --- | --- |\n| 1 2 | x |  |\n|  |  |  |"
	if got != want {
		t.Errorf("table:\n%q\nwant\n%q", got, want)
	}
	if Table(nil) != "" || Table([][]string{{}}) != "" {
		t.Error("no cells is no table")
	}
}
