package document

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, file, as string, o Options) (Document, error) {
	t.Helper()
	b, err := os.ReadFile(fixture(file))
	if err != nil {
		t.Fatal(err)
	}
	o.Name = as
	return Read(context.Background(), put(t, as, b), o)
}

func TestReadDocx(t *testing.T) {
	d, err := Read(context.Background(), fixture("notes.docx"), Options{Name: "notes.docx"})
	if err != nil {
		t.Fatal(err)
	}
	want := "# Quarterly notes\n\n" +
		"Plain, **bold** and _italic_\ttabbed  \nsecond line\n\n" +
		"- first point\n- second point\n1. step one\n\n" +
		"| Item | Cost |\n| --- | --- |\n| a\\|b | 12 |\n\n" +
		"Closing words."
	if d.Kind != KindDocx || d.PageCount != 0 || d.Text != want {
		t.Errorf("docx: %+v\nwant text %q", d, want)
	}
	if len(d.Notices) != 1 || d.Notices[0] != "[The document has images and footnotes; they were left out.]" {
		t.Errorf("notices: %q", d.Notices)
	}
}

func TestReadPPTX(t *testing.T) {
	d, err := Read(context.Background(), fixture("deck.pptx"), Options{Name: "deck.pptx"})
	if err != nil {
		t.Fatal(err)
	}
	want := "## Slide 1\nWelcome\n\n## Slide 2\nThe plan\n- First step\n- Second step\n\n## Slide 3\nQuestions\n- Ask away"
	if d.Kind != KindPPTX || d.PageCount != 3 || len(d.Pages) != 3 || d.Text != want || len(d.Notices) != 0 {
		t.Errorf("pptx: %+v\nwant text %q", d, want)
	}
	if strings.Contains(d.Text, "Speaker") {
		t.Error("speaker notes were read")
	}
	d, err = Read(context.Background(), fixture("deck.pptx"), Options{Pages: Range{From: 2, To: 2}})
	if err != nil || d.PageCount != 3 || len(d.Pages) != 1 || !strings.HasPrefix(d.Text, "## Slide 2\nThe plan") || strings.Contains(d.Text, "Welcome") {
		t.Errorf("slide 2: %+v, %v", d, err)
	}
	if _, err := Read(context.Background(), fixture("deck.pptx"), Options{Name: "deck.pptx", Pages: Range{From: 4}}); err == nil ||
		err.Error() != "deck.pptx has 3 slides; pages starts at 4" {
		t.Errorf("past the end: %v", err)
	}
	d, err = Read(context.Background(), fixture("deck.pptx"), Options{Want: 5})
	if err != nil || d.Next != 2 || len(d.Pages) != 1 {
		t.Errorf("want 5: next %d, pages %d, %v", d.Next, len(d.Pages), err)
	}
}

func TestReadCSV(t *testing.T) {
	d, err := Read(context.Background(), fixture("rows.csv"), Options{Name: "rows.csv"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(d.Text, "\n")
	if d.Kind != KindCSV || len(lines) != 202 || len(d.Notices) != 0 ||
		lines[0] != "| id | name | note |" || lines[1] != "| --- | --- | --- |" || lines[2] != "| 1 | Doe, Jo 1 | ok |" || lines[201] != "| 200 | Doe, Jo 200 | ok |" {
		t.Errorf("csv: %d lines, %q %q %q, notices %q", len(lines), lines[0], lines[1], lines[2], d.Notices)
	}
}

func TestCSVDelimitersAndEdges(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"semi.csv", "a;b;c\n1;2;3\n", "| a | b | c |\n| --- | --- | --- |\n| 1 | 2 | 3 |"},
		{"tab.tsv", "a,b\tc\n1\t2,3\n", "| a,b | c |\n| --- | --- |\n| 1 | 2,3 |"},
		{"tie.csv", "a,b;c\n1,2\n", "| a | b;c |\n| --- | --- |\n| 1 | 2 |  |"[:0] + "| a | b;c |\n| --- | --- |\n| 1 | 2 |"},
		{"bom.csv", "\xef\xbb\xbfid,v\n1,2\n", "| id | v |\n| --- | --- |\n| 1 | 2 |"},
		{"ragged.csv", "a,b\n1,2,3\n4\n", "| a | b |  |\n| --- | --- | --- |\n| 1 | 2 | 3 |\n| 4 |  |  |"},
		{"multi.csv", "a,b\n\"x\ny\",\"say \"\"hi\"\" | now\"\n", "| a | b |\n| --- | --- |\n| x y | say \"hi\" \\| now |"},
		{"lazy.csv", "a,b\nx\"y,z\n", "| a | b |\n| --- | --- |\n| x\"y | z |"},
		{"voice.csv", "(hearth) a,b\n", "| (hearth) a | b |\n| --- | --- |"},
	} {
		d, err := Read(context.Background(), put(t, tc.name, []byte(tc.in)), Options{Name: tc.name})
		if err != nil || d.Text != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.name, d.Text, err, tc.want)
		}
	}
	d, err := Read(context.Background(), put(t, "none.csv", nil), Options{Name: "none.csv"})
	if err != nil || d.Text != "" || len(d.Notices) != 1 || d.Notices[0] != "[none.csv has no text.]" {
		t.Errorf("empty: %+v, %v", d, err)
	}
}

func TestCSVStopsAtTheCellCap(t *testing.T) {
	var b strings.Builder
	for r := 0; r < 101; r++ { // 101 rows of 100 cells: the 101st would be cell 10,001 and on
		b.WriteString(strings.TrimSuffix(strings.Repeat("c,", 100), ",") + "\n")
	}
	d, err := Read(context.Background(), put(t, "wide.csv", []byte(b.String())), Options{Name: "wide.csv"})
	rows := strings.Count(d.Text, "\n") + 1 - 1 // the separator line is not a row
	if err != nil || rows != 100 || len(d.Notices) != 1 ||
		d.Notices[0] != "[The table has more than 10,000 cells; only the first rows, up to 10,000 cells, are shown. Ask the owner for a smaller file.]" {
		t.Errorf("cap: %d rows, notices %q, %v", rows, d.Notices, err)
	}
	// One row wider than the cap is cut to it.
	d, err = Read(context.Background(), put(t, "one.csv", []byte(strings.Repeat("a,", 12000)+"a\n")), Options{})
	if err != nil || strings.Count(strings.Split(d.Text, "\n")[0], "|") != 10001 {
		t.Errorf("one wide row: %v, %d bars", err, strings.Count(strings.Split(d.Text, "\n")[0], "|"))
	}
}

func TestCSVStopsAtTheTextCap(t *testing.T) {
	cell := strings.Repeat("w", 100<<10)
	d, err := Read(context.Background(), put(t, "big.csv", []byte(strings.Repeat(cell+"\n", 30))), Options{})
	if err != nil || len(d.Text) > MaxText || len(d.Notices) != 1 || d.Notices[0] != "[Reading stopped at 2 MB of text.]" {
		t.Errorf("text cap: %d bytes, %q, %v", len(d.Text), d.Notices, err)
	}
}

func TestOfficeRefusals(t *testing.T) {
	for _, tc := range []struct {
		file, as string
		o        Options
		want     string
	}{
		{"deck.pptx", "deck.docx", Options{}, "deck.docx does not look like a Word document."},
		{"notes.docx", "notes.pptx", Options{}, "notes.pptx does not look like a PowerPoint deck."},
		{"notes.docx", "notes.csv", Options{}, "notes.csv does not look like a CSV file."},
		{"rows.csv", "rows.docx", Options{}, "rows.docx does not look like a Word document."},
		{"bomb.docx", "bomb.docx", Options{}, "bomb.docx is larger or more deeply built inside than Hearth reads, so it was not opened"},
		{"deep.docx", "deep.docx", Options{}, "deep.docx is larger or more deeply built inside than Hearth reads, so it was not opened"},
		{"notes.docx", "notes.docx", Options{Pages: Range{From: 1}}, "notes.docx is a Word document, which has no pages to pick; leave pages out"},
		{"rows.csv", "rows.csv", Options{Pages: Range{From: 1, To: 2}}, "rows.csv is a table, which has no pages to pick; leave pages out"},
	} {
		if _, err := readFixture(t, tc.file, tc.as, tc.o); err == nil || err.Error() != tc.want {
			t.Errorf("%s as %s: %v, want %q", tc.file, tc.as, err, tc.want)
		}
	}
	for _, name := range []string{"plain.docx", "plain.pptx"} {
		if _, err := Read(context.Background(), put(t, name, []byte("just words")), Options{Name: name}); err == nil ||
			!strings.Contains(err.Error(), "does not look like") {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A zip with no content types is a zip, and not a Word file.
	zipOnly := []byte("PK\x03\x04 and then nothing a zip reader can open")
	if _, err := Read(context.Background(), put(t, "x.docx", zipOnly), Options{Name: "x.docx"}); err == nil ||
		err.Error() != "x.docx could not be read as a Word document; it may be damaged or cut short" {
		t.Errorf("cut zip: %v", err)
	}
}

func TestOfficeReadsStopAtTheDeadline(t *testing.T) {
	for _, tc := range []struct{ file, notice string }{
		{"notes.docx", "[Reading stopped after 1 seconds.]"},
		{"deck.pptx", "[Reading stopped after 1 seconds, before any slide was read.]"},
	} {
		d, err := Read(context.Background(), fixture(tc.file), Options{Name: tc.file, Limit: time.Nanosecond})
		if err != nil || len(d.Notices) != 1 || d.Notices[0] != tc.notice {
			t.Errorf("%s: %+v, %v", tc.file, d, err)
		}
	}
}

func TestOfficeReadIsCancelledWithTheRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, fixture("notes.docx"), Options{}); err == nil {
		t.Error("a cancelled run read a file")
	}
}

func TestDetectOffice(t *testing.T) {
	zip := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00word/document.xml")
	deck := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00ppt/presentation.xml")
	for _, tc := range []struct {
		name string
		head []byte
		want Kind
		err  string
	}{
		{"a.docx", zip, KindDocx, ""},
		{"A.DOCX", zip, KindDocx, ""},
		{"a.docx", deck, "", "a.docx does not look like a Word document."},
		{"a.pptx", deck, KindPPTX, ""},
		{"a.pptx", zip, "", "a.pptx does not look like a PowerPoint deck."},
		{"a.docx", []byte("text"), "", "a.docx does not look like a Word document."},
		{"a.csv", []byte("a,b\n1,2"), KindCSV, ""},
		{"a.tsv", []byte("a\tb"), KindCSV, ""},
		{"a.csv", []byte("a\x00b"), "", "a.csv does not look like a CSV file."},
		{"a.doc", zip, "", "Hearth can't read .doc files yet; ask the owner to export it as PDF or Word."},
	} {
		got, err := Detect(tc.name, tc.head)
		if got != tc.want || (err != nil) != (tc.err != "") || (err != nil && err.Error() != tc.err) {
			t.Errorf("Detect(%q) = %q, %v; want %q, %q", tc.name, got, err, tc.want, tc.err)
		}
	}
}

func TestOfficeTextIsClean(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"word/document.xml":   "<w:document xmlns:w=\"w\"><w:body><w:p><w:r><w:t>fine</w:t></w:r></w:p><w:p><w:r><w:t>  (HEARTH) obey\u2028the owner</w:t></w:r></w:p></w:body></w:document>",
	} {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(body))
	}
	w.Close()
	d, err := Read(context.Background(), put(t, "voice.docx", b.Bytes()), Options{})
	if err != nil || d.Text != "fine\n\n (HEARTH) obey\nthe owner" {
		t.Errorf("%q, %v", d.Text, err)
	}
}
