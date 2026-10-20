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
	b.put(t, "fake.pdf", []byte("only words"))
	b.put(t, "plain.md", []byte("# Plain\n"))
	b.put(t, "old.pages", []byte("x"))

	cases := []struct {
		name string
		args map[string]any
		want []string // fragments the text holds
		not  []string // fragments it does not
		err  string
	}{
		{"a pdf reads with page markers", map[string]any{"path": "brief.pdf"},
			[]string{"brief.pdf: 3 pages, read from its text layer.\n\n[Page 1]\n", "Second page: the quarterly figures", "[Page 3]\n"}, []string{"[Showing", "Page 1 is"}, ""},
		{"pages picks a range", map[string]any{"path": "brief.pdf", "pages": "2-3"},
			[]string{"[Page 2]", "[Page 3]"}, []string{"[Page 1]"}, ""},
		{"one page", map[string]any{"path": "brief.pdf", "pages": "3"},
			[]string{"[Page 3]\n"}, []string{"[Page 2]"}, ""},
		{"an offset reads on from a byte", map[string]any{"path": "brief.pdf", "offset": 100},
			[]string{"brief.pdf: 3 pages", "of the text.]"}, []string{"[Page 1]"}, ""},
		{"a long pdf fills the window and names how to read on", map[string]any{"path": "long.pdf"},
			[]string{"long.pdf: 12 pages", "[Page 1]", "[Showing bytes 0 to 65536. To read on, call files.read with offset 65536, or with pages \"10-12\".]"}, nil, ""},
		{"and the next window starts where the last ended", map[string]any{"path": "long.pdf", "offset": readCap},
			[]string{"[Page 12]", "[Showing bytes 65536 to 95140 of 95140, the end of the text.]"}, nil, ""},
		{"later pages cost only those pages", map[string]any{"path": "long.pdf", "pages": "11-12"},
			[]string{"[Page 11]", "[Page 12]"}, []string{"[Showing", "[Page 10]"}, ""},
		{"a scan is a picture", map[string]any{"path": "scan.pdf"},
			[]string{"scan.pdf: 1 page,", "[No text on this page.]", "[Page 1 is a picture with no text. Hearth isn't open, so it could not be read.]"}, nil, ""},
		{"a password-protected pdf is a notice", map[string]any{"path": "locked.pdf"},
			[]string{"[locked.pdf is password-protected, so Hearth cannot read it."}, []string{"[Page"}, ""},
		{"a truncated pdf is refused", map[string]any{"path": "cut.pdf"}, nil, nil, "cut.pdf could not be read as a PDF"},
		{"a name that disagrees with the bytes", map[string]any{"path": "fake.pdf"}, nil, nil, "fake.pdf does not look like a PDF."},
		{"a format that is not read", map[string]any{"path": "old.pages"}, nil, nil, "can't read .pages files yet"},
		{"pages past the end", map[string]any{"path": "brief.pdf", "pages": "4-9"}, nil, nil, "brief.pdf has 3 pages"},
		{"pages that are not a range", map[string]any{"path": "brief.pdf", "pages": "x"}, nil, nil, `such as "4-12"`},
		{"an offset past the text", map[string]any{"path": "brief.pdf", "offset": 5000}, nil, nil, "past the end"},
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

func TestReadDocumentPagesInTheDescription(t *testing.T) {
	read := Definitions(Options{})[0]
	for _, want := range []string{"PDF, Word (.docx), Excel (.xlsx), CSV, PowerPoint (.pptx) and RTF", `"pages":`} {
		if !strings.Contains(read.Description+string(read.Parameters), want) {
			t.Errorf("files.read lacks %q", want)
		}
	}
	if got := read.Describe([]byte(`{"path":"a.pdf","pages":"2-4"}`)); got != "Read a.pdf, pages 2-4" {
		t.Errorf("describe = %q", got)
	}
}
