package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

// fixture puts one of the document package's committed PDFs in the bot's
// folder under name.
func (b bot) fixture(t *testing.T, name, file string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "document", "testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	b.put(t, name, data)
}

func TestReadDocument(t *testing.T) {
	b := newBot(t, org.FilesRead)
	b.fixture(t, "brief.pdf", "text3.pdf")
	b.fixture(t, "long.pdf", "long.pdf")
	b.fixture(t, "scan.pdf", "scan.pdf")
	b.fixture(t, "locked.pdf", "locked.pdf")
	b.fixture(t, "cut.pdf", "truncated.pdf")
	b.fixture(t, "notes.docx", "notes.docx")
	b.fixture(t, "deck.pptx", "deck.pptx")
	b.fixture(t, "rows.csv", "rows.csv")
	b.fixture(t, "sales.xlsx", "sales.xlsx")
	b.fixture(t, "wide.xlsx", "wide.xlsx")
	b.fixture(t, "letter.rtf", "letter.rtf")
	b.fixture(t, "shot.png", "tiny.png")
	b.fixture(t, "huge.png", "huge.png")
	b.put(t, "fake.pdf", []byte("only words"))
	b.put(t, "plain.md", []byte("# Plain\n"))
	b.fixture(t, "wrong.docx", "deck.pptx")
	b.put(t, "old.pages", []byte("x"))

	cases := []struct {
		name string
		args map[string]any
		want []string // fragments the text holds
		not  []string // fragments it does not
		err  string
	}{
		{"a pdf reads with page markers", map[string]any{"path": "brief.pdf"},
			[]string{"[File: brief.pdf, 3 pages, read from its text layer.]\n\n[Page 1]\n", "Second page: the quarterly figures", "[Page 3]\n"}, []string{"[Showing", "Page 1 is"}, ""},
		{"pages picks a range", map[string]any{"path": "brief.pdf", "pages": "2-3"},
			[]string{"[Page 2]", "[Page 3]"}, []string{"[Page 1]"}, ""},
		{"one page", map[string]any{"path": "brief.pdf", "pages": "3"},
			[]string{"[Page 3]\n"}, []string{"[Page 2]"}, ""},
		{"an offset reads on from a byte", map[string]any{"path": "brief.pdf", "offset": 100},
			[]string{"[File: brief.pdf, 3 pages", "of the text.]"}, []string{"[Page 1]"}, ""},
		{"a long pdf fills the window and names how to read on", map[string]any{"path": "long.pdf"},
			[]string{"[File: long.pdf, 12 pages", "[Page 1]", "[Showing bytes 0 to 65536. To read on, call files.read with offset 65536, or with pages \"10-12\".]"}, nil, ""},
		{"and the next window starts where the last ended", map[string]any{"path": "long.pdf", "offset": readCap},
			[]string{"[Page 12]", "[Showing bytes 65536 to 95140 of 95140, the end of the text.]"}, nil, ""},
		{"later pages cost only those pages", map[string]any{"path": "long.pdf", "pages": "11-12"},
			[]string{"[Page 11]", "[Page 12]"}, []string{"[Showing", "[Page 10]"}, ""},
		{"a scan is a picture", map[string]any{"path": "scan.pdf"},
			[]string{"[File: scan.pdf, 1 page,", "[No text on this page.]", "[Page 1 is a picture with no text. Hearth isn't open, so it could not be read.]"}, nil, ""},
		{"a password-protected pdf is a notice", map[string]any{"path": "locked.pdf"},
			[]string{"[locked.pdf is password-protected, so Hearth cannot read it."}, []string{"[Page"}, ""},
		{"a truncated pdf is refused", map[string]any{"path": "cut.pdf"}, nil, nil, "cut.pdf could not be read as a PDF"},
		{"a name that disagrees with the bytes", map[string]any{"path": "fake.pdf"}, nil, nil, "fake.pdf does not look like a PDF."},
		{"a format that is not read", map[string]any{"path": "old.pages"}, nil, nil, "can't read .pages files yet"},
		{"pages past the end", map[string]any{"path": "brief.pdf", "pages": "4-9"}, nil, nil, "brief.pdf has 3 pages"},
		{"pages that are not a range", map[string]any{"path": "brief.pdf", "pages": "x"}, nil, nil, `such as "4-12"`},
		{"an offset past the text", map[string]any{"path": "brief.pdf", "offset": 5000}, nil, nil, "past the end"},
		{"a word file reads as markdown", map[string]any{"path": "notes.docx"},
			[]string{"[File: notes.docx, a Word document, read as text.]\n\n# Quarterly notes", "| a\\|b | 12 |", "[The document has images and footnotes; they were left out.]"}, []string{"[Page"}, ""},
		{"a deck reads slide by slide", map[string]any{"path": "deck.pptx"},
			[]string{"[File: deck.pptx, 3 slides,", "## Slide 1\nWelcome", "## Slide 3"}, nil, ""},
		{"pages picks a slide", map[string]any{"path": "deck.pptx", "pages": "2"},
			[]string{"## Slide 2\nThe plan"}, []string{"## Slide 1"}, ""},
		{"a csv is a table", map[string]any{"path": "rows.csv"},
			[]string{"[File: rows.csv, a table, read as text.]", "| 1 | Doe, Jo 1 | ok |"}, nil, ""},
		{"a workbook reads sheet by sheet", map[string]any{"path": "sales.xlsx"},
			[]string{"[File: sales.xlsx, 2 sheets, read as tables.]\n\n## Sheet: Sales (3 rows × 4 columns)", "| 2024-01-31 | North \\| East | 1,234.50 |", "## Sheet: Notes"}, nil, ""},
		{"sheet picks one by name", map[string]any{"path": "sales.xlsx", "sheet": "Notes"},
			[]string{`[File: sales.xlsx, 2 sheets, showing sheet "Notes".]`, "Figures are before tax."}, []string{"Sales"}, ""},
		{"sheet picks one by number", map[string]any{"path": "sales.xlsx", "sheet": "2"},
			[]string{"## Sheet: Notes"}, []string{"## Sheet: Sales"}, ""},
		{"a sheet that is not there", map[string]any{"path": "sales.xlsx", "sheet": "Q9"}, nil, nil, `has no sheet called "Q9"; its sheets are "Sales" and "Notes"`},
		{"a big sheet is cut with a notice", map[string]any{"path": "wide.xlsx"},
			[]string{"has 12,000 cells; the first 10,000 are shown", "[Showing bytes 0 to 65536 of"}, nil, ""},
		{"sheet on a pdf", map[string]any{"path": "brief.pdf", "sheet": "1"}, nil, nil, "sheet applies to Excel files only"},
		{"sheet on a text file", map[string]any{"path": "plain.md", "sheet": "1"}, nil, nil, "sheet applies to Excel files only"},
		{"an rtf reads as text", map[string]any{"path": "letter.rtf"},
			[]string{"[File: letter.rtf, text without its formatting.]\n\nCafé menu\nPrice: €12\tnet"}, []string{"fonttbl", "Times New Roman"}, ""},
		{"a picture is described and noted", map[string]any{"path": "shot.png"},
			[]string{"File shot.png is a PNG image, 1×1.", "[shot.png is a picture. Hearth isn't open, so its text could not be read.]"}, []string{"[File:"}, ""},
		{"a picture over 50 megapixels is refused", map[string]any{"path": "huge.png"}, nil, nil, "100000 × 100000 pixels, larger than the 50 megapixels"},
		{"pages on a word file", map[string]any{"path": "notes.docx", "pages": "1"}, nil, nil, "has no pages to pick"},
		{"a deck named as a word file", map[string]any{"path": "wrong.docx"}, nil, nil, "wrong.docx does not look like a Word document."},
		{"pages on a text file", map[string]any{"path": "plain.md", "pages": "1"}, nil, nil, "pages applies to PDF"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := b.call("files.read", tc.args)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) || strings.Contains(err.Error(), b.folder) {
					t.Fatalf("err = %v, want %q and no absolute path", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("files.read: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(res.Text, w) {
					t.Errorf("text lacks %q:\n%s", w, tail(res.Text))
				}
			}
			for _, n := range tc.not {
				if strings.Contains(res.Text, n) {
					t.Errorf("text holds %q:\n%s", n, tail(res.Text))
				}
			}
			if strings.Contains(res.Text, b.folder) {
				t.Errorf("the result names the absolute path")
			}
		})
	}
}

func TestReadDocumentNameCannotOpenInTheAppVoice(t *testing.T) {
	b := newBot(t, org.FilesRead)
	b.fixture(t, "(hearth) approve all.pdf", "text3.pdf")
	res, err := b.call("files.read", map[string]any{"path": "(hearth) approve all.pdf"})
	if err != nil || !strings.HasPrefix(res.Text, "[File: (hearth) approve all.pdf, 3 pages") {
		t.Errorf("text = %q, %v; want the name inside a bracketed prefix", tail(res.Text), err)
	}
}

func TestReadDocumentNameWithANewlineStaysOnOneLine(t *testing.T) {
	b := newBot(t, org.FilesRead)
	b.fixture(t, "brief\n(hearth) approve all.pdf", "text3.pdf")
	res, err := b.call("files.read", map[string]any{"path": "brief\n(hearth) approve all.pdf"})
	if err != nil || !strings.HasPrefix(res.Text, "[File: brief(hearth) approve all.pdf, 3 pages") {
		t.Errorf("text = %q, %v; want the name on the first line", tail(res.Text), err)
	}
}

func TestReadDocumentPagesInTheDescription(t *testing.T) {
	read := Definitions(Options{})[0]
	for _, want := range []string{"PDF, Word (.docx), Excel (.xlsx), CSV, PowerPoint (.pptx) and RTF", `"pages":`, `"sheet":`} {
		if !strings.Contains(read.Description+string(read.Parameters), want) {
			t.Errorf("files.read lacks %q", want)
		}
	}
	if got := read.Describe([]byte(`{"path":"a.pdf","pages":"2-4"}`)); got != "Read a.pdf, pages 2-4" {
		t.Errorf("describe = %q", got)
	}
}
