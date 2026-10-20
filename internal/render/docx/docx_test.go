package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/document"
)

// folder is an Opener over a few images made here. A path with .. in it is
// refused, as the caller's containment would refuse it.
func folder(t *testing.T) Opener {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for x := range 40 {
		img.Set(x, x/2, color.NRGBA{R: 200, A: 128})
	}
	var p, j bytes.Buffer
	if err := png.Encode(&p, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&j, img, nil); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"pic.png": p.Bytes(), "photo one.jpg": j.Bytes()}
	return func(rel string) (io.ReadCloser, error) {
		if strings.Contains(rel, "..") {
			return nil, errors.New("outside")
		}
		b, ok := files[rel]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "every.markdown"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func write(t *testing.T, src []byte, o Options) ([]byte, Report) {
	t.Helper()
	out, rep, err := Write(context.Background(), src, o)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rep.Bytes != len(out) || !bytes.HasPrefix(out, []byte("PK\x03\x04")) {
		t.Fatalf("Report.Bytes = %d for %d bytes starting %q", rep.Bytes, len(out), out[:min(len(out), 4)])
	}
	return out, rep
}

// parts reads every part of a Word file by name.
func parts(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = string(b)
	}
	return out
}

// read reads a Word file back through the document reader.
func read(t *testing.T, data []byte) document.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.docx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Read(context.Background(), path, document.Options{})
	if err != nil {
		t.Fatalf("document.Read: %v", err)
	}
	return doc
}

func TestWrite_ReproducibleAndReadsBack(t *testing.T) {
	o := Options{Name: "report.md", Image: folder(t)}
	first, rep := write(t, fixture(t), o)
	second, _ := write(t, fixture(t), o)
	if !bytes.Equal(first, second) {
		t.Error("writing the same source twice gave different bytes")
	}
	doc := read(t, first)
	if doc.Kind != document.KindDocx {
		t.Fatalf("Kind = %q, want docx", doc.Kind)
	}
	var headings []string
	for _, line := range strings.Split(doc.Text, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	want := []string{"# Quarterly report", "## Figures", "### A list", "#### Quoted", "##### Code", "###### Last heading"}
	if !slices.Equal(headings, want) {
		t.Errorf("headings = %q, want %q", headings, want)
	}
	for _, w := range []string{
		"**bold**", "_italic_", "**_both_**",
		"Ελληνικά, Русский and Français",
		"- First point", "- Second point with a nested list:", "  1. one", "  1. two",
		"| **Region** | **Revenue** | **Change** |", "| North | 1,204 | +4% |",
		"func main() {", "A quote that runs long",
	} {
		if !strings.Contains(doc.Text, w) {
			t.Errorf("the text read back has no %q:\n%s", w, doc.Text)
		}
	}
	if strings.Contains(doc.Text, "raw html") || strings.Contains(doc.Text, "<b>") {
		t.Error("raw HTML reached the Word file")
	}
	for _, w := range []string{
		"[report.md had HTML in it; it was left out of the Word file.]",
		"[report.md has images that could not be used",
		"[report.md has images from the web",
	} {
		if !slices.ContainsFunc(rep.Notices, func(n string) bool { return strings.HasPrefix(n, w) }) {
			t.Errorf("notices %q lack %q", rep.Notices, w)
		}
	}
}

func TestWrite_Parts(t *testing.T) {
	out, _ := write(t, fixture(t), Options{Name: "report.md", Image: folder(t)})
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if !f.Modified.Equal(zipTime()) {
			t.Errorf("%s has the time %v, want the fixed %v", f.Name, f.Modified, zipTime())
		}
	}
	if !slices.IsSorted(names) {
		t.Errorf("parts are not sorted: %q", names)
	}
	for _, w := range []string{"word/media/image1.png", "word/media/image2.jpg"} {
		if !slices.Contains(names, w) {
			t.Errorf("no %s in %q", w, names)
		}
	}
	all := parts(t, out)
	for name, body := range all {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		d := xml.NewDecoder(strings.NewReader(body))
		for {
			if _, err := d.Token(); err != nil {
				if !errors.Is(err, io.EOF) {
					t.Errorf("%s is not well-formed XML: %v", name, err)
				}
				break
			}
		}
	}
	doc := all["word/document.xml"]
	for _, w := range []string{`w:val="Title"`, `w:val="Heading2"`, `w:val="ListBullet"`, `w:val="ListNumber"`, `w:val="Quote"`, `w:val="Code"`, `w:val="TableGrid"`, `<w:tblHeader/>`, `<w:strike/>`, `<w:drawing>`} {
		if !strings.Contains(doc, w) {
			t.Errorf("document.xml has no %s", w)
		}
	}
}

func TestWrite_LinksAreTextOnly(t *testing.T) {
	src := []byte("See [the plan](https://example.com/plan?a=1&b=2), [https://same.example](https://same.example) and <https://example.com/auto>.\n")
	out, _ := write(t, src, Options{Name: "a.md"})
	all := parts(t, out)
	for name, body := range all {
		if strings.Contains(strings.ToLower(body), "hyperlink") || strings.Contains(body, "TargetMode") {
			t.Errorf("%s makes a hyperlink", name)
		}
	}
	for _, w := range []string{"the plan</w:t>", "[https://example.com/plan?a=1&amp;b=2]", "https://example.com/auto"} {
		if !strings.Contains(all["word/document.xml"], w) {
			t.Errorf("document.xml lacks %q", w)
		}
	}
	text := read(t, out).Text
	if strings.Count(text, "https://same.example") != 1 {
		t.Errorf("a link whose text is its address repeats it: %q", text)
	}
	if !strings.Contains(text, "the plan [https://example.com/plan?a=1&b=2]") {
		t.Errorf("text %q lacks the link as text with its address in brackets", text)
	}
}

func TestWrite_Lists(t *testing.T) {
	src := []byte("3. three\n4. four\n\ntext\n\n1. again\n\n- a\n  - b\n    - c\n\n-\n\n- [ ] open\n- [x] done\n")
	out, _ := write(t, src, Options{Name: "a.md"})
	all := parts(t, out)
	n := all["word/numbering.xml"]
	for _, w := range []string{`w:numId="2"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="3"/>`, `w:numId="3"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/>`} {
		if !strings.Contains(n, w) {
			t.Errorf("numbering.xml lacks %s", w)
		}
	}
	if !strings.Contains(all["word/document.xml"], `<w:ilvl w:val="2"/>`) {
		t.Error("a list nested twice is not on level 2")
	}
	text := read(t, out).Text
	for _, w := range []string{"1. three", "    - c", "\u2610 open", "\u2611 done"} {
		if !strings.Contains(text, w) {
			t.Errorf("text %q lacks %q", text, w)
		}
	}
}

func TestWrite_ManyNumberedLists(t *testing.T) {
	src := []byte(strings.Repeat("1. a\n\ntext\n\n", maxNumbered+5))
	out, rep := write(t, src, Options{Name: "a.md"})
	if got := strings.Count(parts(t, out)["word/numbering.xml"], "<w:num "); got != maxNumbered+1 {
		t.Errorf("%d numberings, want %d", got, maxNumbered+1)
	}
	if !slices.Contains(rep.Notices, "[a.md has more than 1,000 numbered lists; the rest are bulleted in the Word file.]") {
		t.Errorf("notices = %q", rep.Notices)
	}
}

func TestWrite_TextIsCleaned(t *testing.T) {
	src := []byte("a\x00b\u202ec\x1bd \u2028e\n\n```\n\ttabbed\x07\n```\n")
	out, _ := write(t, src, Options{Name: "a.md"})
	body := parts(t, out)["word/document.xml"]
	for _, bad := range []string{"\x00", "\u202e", "\x1b", "\x07", "\u2028", "\t"} {
		if strings.Contains(body, bad) {
			t.Errorf("document.xml carries %q", bad)
		}
	}
	if !strings.Contains(read(t, out).Text, "bcd  e") {
		t.Error("the cleaned text is not there")
	}
}

func TestWrite_PaperAndNoImages(t *testing.T) {
	out, rep := write(t, []byte("![x](pic.png)\n"), Options{Name: "a.md", A4: true})
	body := parts(t, out)["word/document.xml"]
	if !strings.Contains(body, `w:w="11906" w:h="16838"`) {
		t.Error("A4 was not set")
	}
	if strings.Contains(body, "<w:drawing>") || !strings.Contains(body, "[Image: x]") || len(rep.Notices) != 1 {
		t.Errorf("an image with no Opener should be a placeholder with a notice: %q %q", body, rep.Notices)
	}
}

func TestWrite_Refusals(t *testing.T) {
	if _, _, err := Write(context.Background(), bytes.Repeat([]byte("a"), MaxSource+1), Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a source over the cap: %v", err)
	}
	if _, _, err := Write(context.Background(), []byte(strings.Repeat("[", 20000)), Options{}); err == nil {
		t.Error("source the strict reader refuses was parsed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Write(ctx, fixture(t), Options{Name: "a.md"}); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled context: %v", err)
	}
}

func TestWrite_DeepNestingStops(t *testing.T) {
	src := []byte(strings.Repeat("> ", 100) + "deep\n")
	_, rep := write(t, src, Options{Name: "a.md"})
	if !slices.Contains(rep.Notices, "[a.md nests lists or quotes too deeply; the innermost were left out.]") {
		t.Errorf("notices = %q", rep.Notices)
	}
}
