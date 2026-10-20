package paper

import (
	"bytes"
	"context"
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
	"time"

	"github.com/rfizzle/hearth/internal/document/pdftext"
)

var modified = time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC)

// folder is an Opener over a few images made here, recording what it was
// asked for. A path with .. in it is refused, as the caller's containment
// would refuse it.
type folder struct {
	files map[string][]byte
	asked []string
}

func (f *folder) open(rel string) (io.ReadCloser, error) {
	f.asked = append(f.asked, rel)
	if strings.Contains(rel, "..") {
		return nil, errors.New("outside")
	}
	b, ok := f.files[rel]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func images(t *testing.T) *folder {
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
	return &folder{files: map[string][]byte{"pic.png": p.Bytes(), "photo one.jpg": j.Bytes()}}
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "every.markdown"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func render(t *testing.T, src []byte, o Options) ([]byte, Report) {
	t.Helper()
	out, rep, err := Markdown(context.Background(), src, o)
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if rep.Bytes != len(out) || !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("Report.Bytes = %d for %d bytes starting %q", rep.Bytes, len(out), out[:min(len(out), 8)])
	}
	return out, rep
}

// text reads every page of a PDF back through the document reader.
func text(t *testing.T, pdf []byte) []string {
	t.Helper()
	r, err := pdftext.Open(context.Background(), bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatalf("reading the PDF back: %v", err)
	}
	var pages []string
	for n := 1; n <= r.NumPage(); n++ {
		p, err := r.Page(context.Background(), n)
		if err != nil {
			t.Fatalf("page %d: %v", n, err)
		}
		pages = append(pages, strings.Join(strings.Fields(p), " "))
	}
	return pages
}

func TestMarkdownIsReproducible(t *testing.T) {
	src := fixture(t)
	o := Options{Name: "every.md", Modified: modified, Image: images(t).open}
	first, rep := render(t, src, o)
	o.Image = images(t).open
	second, _ := render(t, src, o)
	if !bytes.Equal(first, second) {
		t.Fatal("the same source rendered twice gave different bytes")
	}
	for _, want := range []string{"/Producer (Hearth)", "/CreationDate (D:20261020093000)", "/ModDate (D:20261020093000)"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("the PDF is missing %s", want)
		}
	}
	want := []string{
		"[every.md had HTML in it; it was left out of the PDF.]",
		"[every.md has images that could not be used, marked in the PDF; an image must be a PNG or JPEG file inside this folder, at most 20 megapixels and 20 to a document.]",
		"[every.md has images from the web; they were left out of the PDF. Save each image in the folder and name it by its path.]",
	}
	if !slices.Equal(rep.Notices, want) {
		t.Errorf("notices:\n%q\nwant\n%q", rep.Notices, want)
	}
	other, _ := render(t, src, Options{Name: "every.md", Modified: modified.Add(time.Second)})
	if bytes.Equal(first, other) {
		t.Error("a different modification time gave the same bytes")
	}
}

func TestMarkdownReadsBack(t *testing.T) {
	pdf, rep := render(t, fixture(t), Options{Name: "every.md", Modified: modified, Image: images(t).open})
	pages := text(t, pdf)
	if len(pages) != rep.Pages {
		t.Fatalf("read %d pages, the report says %d", len(pages), rep.Pages)
	}
	all := strings.Join(pages, "\n")
	at := 0
	for _, h := range []string{"Quarterly report", "Figures", "A list", "Quoted", "Code", "Last heading"} {
		i := strings.Index(all[at:], h)
		if i < 0 {
			t.Fatalf("heading %q is missing or out of order in:\n%s", h, all)
		}
		at += i + len(h)
	}
	for _, want := range []string{
		"bold", "struck", "(https://example.com/q3)", "Ελληνικά", "Русский", "North", "1,204",
		`fmt.Println("hello")`, "[Image: Missing]", "[Image: Outside]", "[Image: From the web]",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("the text read back is missing %q", want)
		}
	}
	for _, gone := range []string{"raw html", "<b>", "[Image: A chart]", "[Image: A photo]"} {
		if strings.Contains(all, gone) {
			t.Errorf("the text read back has %q", gone)
		}
	}
	if !strings.Contains(pages[len(pages)-1], "1 of") && !strings.Contains(pages[len(pages)-1], " of ") {
		t.Errorf("the last page has no page number: %q", pages[len(pages)-1])
	}
}

func TestMarkdownImages(t *testing.T) {
	f := images(t)
	render(t, fixture(t), Options{Name: "every.md", Modified: modified, Image: f.open})
	want := []string{"pic.png", "photo one.jpg", "nowhere.png", "../secret.png"}
	if !slices.Equal(f.asked, want) {
		t.Errorf("opened %q, want %q: a web address must never reach the opener", f.asked, want)
	}

	// Without an opener no file is reached, and every image is a placeholder.
	pdf, _ := render(t, []byte("![Chart](pic.png)\n\n![](/etc/hosts)\n\n![](data:image/png;base64,AAAA)"), Options{Name: "a.md"})
	all := strings.Join(text(t, pdf), "\n")
	if !strings.Contains(all, "[Image: Chart]") || strings.Count(all, "[Image") != 3 {
		t.Errorf("want three placeholders, got:\n%s", all)
	}

	// An image that is not a PNG or JPEG, or claims too many pixels, is left
	// out before it is decoded.
	big := &folder{files: map[string][]byte{
		"note.png": []byte("not an image"),
		"huge.png": pngHeader(10000, 10000),
	}}
	_, rep := render(t, []byte("![](note.png)\n![](huge.png)"), Options{Name: "a.md", Image: big.open})
	if len(rep.Notices) != 1 || !strings.Contains(rep.Notices[0], "could not be used") {
		t.Errorf("notices = %q", rep.Notices)
	}
}

// pngHeader is the start of a PNG claiming w by h pixels, with no pixels.
func pngHeader(w, h int) []byte {
	img := image.NewGray(image.Rect(0, 0, 1, 1))
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	out := b.Bytes()
	// IHDR's width and height follow the 8-byte signature, the length and the type.
	put := func(at, v int) {
		out[at], out[at+1], out[at+2], out[at+3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
	}
	put(16, w)
	put(20, h)
	return out
}

func TestMarkdownStopsAtMaxPages(t *testing.T) {
	src := bytes.Repeat([]byte("A line of its own.\n\n"), 8000)
	pdf, rep := render(t, src, Options{Name: "long.md", Modified: modified})
	if rep.Pages != MaxPages {
		t.Errorf("Pages = %d, want %d", rep.Pages, MaxPages)
	}
	if want := "[long.md runs past 200 pages; the PDF stops at page 200.]"; !slices.Contains(rep.Notices, want) {
		t.Errorf("notices = %q, want %q", rep.Notices, want)
	}
	pages := text(t, pdf)
	if last := pages[len(pages)-1]; !strings.Contains(last, "200 of 200") {
		t.Errorf("the last page's number reads %q", last)
	}
}

func TestMarkdownBounds(t *testing.T) {
	if _, _, err := Markdown(context.Background(), make([]byte, MaxSource+1), Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("an oversized source: %v, want ErrTooLarge", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Markdown(ctx, []byte("# Hi"), Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled render: %v, want context.Canceled", err)
	}
	for name, src := range map[string]string{
		"quotes": strings.Repeat(">", 200000) + " deep",
		"lists":  strings.Repeat("- ", 100000) + "deep",
		"items":  "- a\n" + strings.Repeat("  ", 200) + "- b\n",
	} {
		if _, _, err := Markdown(context.Background(), []byte(src), Options{}); !errors.Is(err, ErrTooDeep) {
			t.Errorf("%s: %v, want ErrTooDeep", name, err)
		}
	}
	for _, src := range []string{strings.Repeat("[a](", 100000), strings.Repeat("[a]: b\n", 20000) + "[a]"} {
		if _, _, err := Markdown(context.Background(), []byte(src), Options{}); !errors.Is(err, ErrTooManyBrackets) {
			t.Errorf("%.12q…: %v, want ErrTooManyBrackets", src, err)
		}
	}
	for name, src := range map[string]string{
		"links":    strings.Repeat("[a](", maxBrackets),
		"nested":   strings.Repeat("> ", 120) + "deep",
		"emphasis": strings.Repeat("*a ", 100000) + strings.Repeat("a* ", 100000),
		"one word": strings.Repeat("x", 500000),
		"a rule":   strings.Repeat("-", 5000),
		"a number": strings.Repeat("7", 5000),
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			_, rep, err := Markdown(context.Background(), []byte(src), Options{Name: "x.md"})
			if err != nil {
				t.Errorf("Markdown: %v", err)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("took %v", d)
			}
			if name == "nested" && (len(rep.Notices) != 1 || !strings.Contains(rep.Notices[0], "too deeply")) {
				t.Errorf("notices = %q", rep.Notices)
			}
		})
	}
}

func TestPaper(t *testing.T) {
	for in, want := range map[string]Paper{"": Letter, "letter": Letter, "A4": A4} {
		if got, err := ParsePaper(in); err != nil || got != want {
			t.Errorf("ParsePaper(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParsePaper("legal"); err == nil {
		t.Error("ParsePaper accepted legal")
	}
	letter, _ := render(t, []byte("# A"), Options{})
	a4, _ := render(t, []byte("# A"), Options{Paper: A4})
	if !bytes.Contains(letter, []byte("/MediaBox [0 0 612.00 792.00]")) || !bytes.Contains(a4, []byte("/MediaBox [0 0 595.28 841.89]")) {
		t.Error("the page sizes are not Letter and A4")
	}
}

func TestClean(t *testing.T) {
	in := "a\tb\x00c" + string(rune(0x202E)) + "d" + string(rune(0x1F642)) + "\n"
	if got := Clean(in); got != "a bcd"+string(rune(0xFFFD)) {
		t.Errorf("Clean = %q", got)
	}
}

// TestDocument draws on a page directly, as a second entry point would: a
// quote's edge across a page break, a rule, and the dates from Finish.
func TestDocument(t *testing.T) {
	d, err := New(A4, Print())
	if err != nil {
		t.Fatal(err)
	}
	if d.PDF() == nil || d.Width() != 595.28-2*72 || d.Full() {
		t.Fatalf("a new A4 document: width %v, full %v", d.Width(), d.Full())
	}
	d.Font("mono", "BIU", 10) // the mono family has no bold or italic face
	d.Ink(d.Style.Secondary)
	d.OpenBar(72)
	for range 100 {
		d.Need(d.Style.Leading)
		d.PDF().CellFormat(0, d.Style.Leading, "line", "", 1, "L", false, 0, "")
	}
	d.CloseBar()
	d.Space(d.Style.Gap)
	d.Rule()
	out, pages, err := d.Finish("A title\x00", time.Time{})
	if err != nil || pages != 3 {
		t.Fatalf("Finish = %d pages, %v", pages, err)
	}
	for _, want := range []string{"/CreationDate (D:19700101000000)", "/Title (\xfe\xff\x00A\x00 \x00t\x00i\x00t\x00l\x00e)"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("the PDF is missing %s", want)
		}
	}
}
