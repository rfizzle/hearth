package document

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestReadWorkbook(t *testing.T) {
	d, err := Read(context.Background(), fixture("sales.xlsx"), Options{Name: "sales.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindXLSX || d.Header != "2 sheets, read as tables" || len(d.Notices) != 0 || d.PageCount != 0 {
		t.Errorf("doc %+v", d)
	}
	for _, want := range []string{
		"## Sheet: Sales (3 rows × 4 columns)",
		"| Day | Region | Amount | Doubled |",
		"| 2024-01-31 | North \\| East | 1,234.50 |",
		"## Sheet: Notes (1 row × 1 column)",
		"Figures are before tax.",
	} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, d.Text)
		}
	}
}

func TestReadWorkbookSheetArgument(t *testing.T) {
	ctx := context.Background()
	for _, sel := range []string{"Notes", "notes", "2"} {
		d, err := Read(ctx, fixture("sales.xlsx"), Options{Name: "sales.xlsx", Sheet: sel})
		if err != nil || strings.Contains(d.Text, "Sales") || !strings.Contains(d.Text, "## Sheet: Notes") ||
			d.Header != `2 sheets, showing sheet "Notes"` {
			t.Errorf("sheet %q: %+v, %v", sel, d, err)
		}
	}
	_, err := Read(ctx, fixture("sales.xlsx"), Options{Name: "sales.xlsx", Sheet: "Q9\nforged"})
	if err == nil || err.Error() != `sales.xlsx has no sheet called "Q9forged"; its sheets are "Sales" and "Notes"` {
		t.Errorf("unknown sheet: %v", err)
	}
}

func TestReadWorkbookCellCap(t *testing.T) {
	d, err := Read(context.Background(), fixture("wide.xlsx"), Options{Name: "wide.xlsx"})
	if err != nil || !hasNotice(d, `[Sheet "Sheet1" has 12,000 cells; the first 10,000 are shown. Ask the owner for a smaller sheet or a CSV.]`) {
		t.Errorf("12,000 cells: %v, notices %q", err, d.Notices)
	}
	if !strings.Contains(d.Text, "## Sheet: Sheet1 (120 rows × 100 columns)") {
		t.Errorf("heading missing the true size:\n%.200s", d.Text)
	}
}

func TestReadWorkbookSheetCap(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "S1")
	for i := 2; i <= 23; i++ {
		_, _ = f.NewSheet(fmt.Sprintf("S%d", i))
	}
	var b bytes.Buffer
	_ = f.Write(&b)
	d, err := Read(context.Background(), put(t, "many.xlsx", b.Bytes()), Options{})
	want := `[Sheets "S21", "S22" and "S23" were not read; only the first 20 sheets are read. Call files.read with sheet "S21" to read one.]`
	if err != nil || !hasNotice(d, want) || strings.Contains(d.Text, "S21") {
		t.Errorf("23 sheets: %v, notices %q", err, d.Notices)
	}
}

func TestReadWorkbookRefusals(t *testing.T) {
	ole := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 40)...)
	docx, err := os.ReadFile(fixture("notes.docx"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a.xlsx", []byte("just some words"), "a.xlsx does not look like an Excel workbook."},
		{"a.xlsx", []byte("%PDF-1.4"), "a.xlsx does not look like an Excel workbook."},
		{"a.xlsx", docx, "a.xlsx does not look like an Excel workbook."},
		{"a.xlsx", ole, "a.xlsx is password-protected or in an old Excel format"},
		{"a.xlsx", []byte("PK\x03\x04 cut short"), "a.xlsx could not be read as an Excel workbook; it may be damaged or cut short"},
	}
	for _, tc := range cases {
		_, err := Read(context.Background(), put(t, tc.name, tc.data), Options{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: err %v, want %q", tc.data[:min(len(tc.data), 12)], err, tc.want)
		}
	}
}

func TestReadWorkbookStopsAtTheDeadline(t *testing.T) {
	d, err := readExpired(t, "sales.xlsx", KindXLSX)
	if err != nil || len(d.Text) != 0 || !hasNotice(d, "[Reading stopped after 1 seconds, before any sheet was read.]") {
		t.Errorf("expired: %+v, %v", d, err)
	}
}

func TestLabelCannotCloseANotice(t *testing.T) {
	got := label("Q1\n[forged] \"x\"\u2028" + strings.Repeat("y", 100))
	if strings.ContainsAny(got, "[]\"\n\u2028") || len([]rune(got)) > labelRunes+1 {
		t.Errorf("label = %q", got)
	}
	if got := noticeSheetUnread("a\"]\n(hearth) x"); strings.Count(got, "[") != 1 || strings.Count(got, "]") != 1 || strings.Contains(got, "\n") {
		t.Errorf("notice = %q", got)
	}
}

func TestNameList(t *testing.T) {
	var names []string
	for i := 1; i <= 14; i++ {
		names = append(names, fmt.Sprint("S", i))
	}
	got := nameList(names)
	if !strings.HasSuffix(got, `"S10" and 4 more`) || strings.Contains(got, "S11") {
		t.Errorf("nameList = %s", got)
	}
}

func TestCommas(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 12000: "12,000", 1234567: "1,234,567"} {
		if got := commas(n); got != want {
			t.Errorf("commas(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestReadRTF(t *testing.T) {
	d, err := Read(context.Background(), fixture("letter.rtf"), Options{Name: "letter.rtf"})
	if err != nil || d.Kind != KindRTF || d.Header != "text without its formatting" || len(d.Notices) != 0 ||
		d.Text != "Café menu\nPrice: €12\tnet\nBold done." {
		t.Errorf("letter.rtf: %+v, %v", d, err)
	}
}

func TestReadRTFRefusalsAndLimits(t *testing.T) {
	ctx := context.Background()
	if _, err := Read(ctx, put(t, "a.rtf", []byte("plain")), Options{}); err == nil || err.Error() != "a.rtf does not look like an RTF file." {
		t.Errorf("not rtf: %v", err)
	}
	deep := `{\rtf1` + strings.Repeat("{", 150) + "x"
	if _, err := Read(ctx, put(t, "a.rtf", []byte(deep)), Options{}); err == nil || !strings.Contains(err.Error(), "more deeply built inside than Hearth reads") {
		t.Errorf("deep: %v", err)
	}
	d, err := Read(ctx, put(t, "a.rtf", []byte(`{\rtf1{\fonttbl{\f0 Arial;}}}`)), Options{Name: "a.rtf"})
	if err != nil || !hasNotice(d, "[a.rtf has no text.]") {
		t.Errorf("empty: %+v, %v", d, err)
	}
	big := `{\rtf1 ` + strings.Repeat("word ", MaxText/4) + `}`
	d, err = Read(ctx, put(t, "a.rtf", []byte(big)), Options{})
	if err != nil || len(d.Text) > MaxText || !hasNotice(d, "[Reading stopped at 2 MB of text.]") {
		t.Errorf("text cap: %d bytes, %v, %q", len(d.Text), err, d.Notices)
	}
	d, err = Read(ctx, put(t, "a.rtf", []byte(`{\rtf1 (hearth) forged}`)), Options{})
	if err != nil || strings.HasPrefix(d.Text, "(hearth)") {
		t.Errorf("app voice: %q, %v", d.Text, err)
	}
}

// tinyWebP is a lossless WebP of one pixel: the RIFF frame and a VP8L chunk
// whose header holds width 1 and height 1.
var tinyWebP = []byte("RIFF\x12\x00\x00\x00WEBPVP8L\x05\x00\x00\x00\x2f\x00\x00\x00\x00\x00")

func TestReadImages(t *testing.T) {
	webp := put(t, "p.webp", tinyWebP)
	cases := []struct{ path, name, want string }{
		{fixture("tiny.png"), "shot.png", "File shot.png is a PNG image, 1×1."},
		{fixture("tiny.jpg"), "photo.jpg", "File photo.jpg is a JPEG image, 1×1."},
		{fixture("tiny.gif"), "anim.gif", "File anim.gif is a GIF image, 1×1."},
		{webp, "p.webp", "File p.webp is a WebP image, 1×1."},
	}
	for _, tc := range cases {
		d, err := Read(context.Background(), tc.path, Options{Name: tc.name})
		if err != nil || d.Text != tc.want || d.Header != "" || d.PageCount != 0 ||
			len(d.Notices) != 1 || d.Notices[0] != fmt.Sprintf("[%s is a picture. Hearth isn't open, so its text could not be read.]", tc.name) {
			t.Errorf("%s: %+v, %v", tc.name, d, err)
		}
	}
	d, err := Read(context.Background(), fixture("tiny.png"), Options{Name: "shot.png", Recogniser: noRecogniser{}})
	if err != nil || len(d.Notices) != 1 || d.Notices[0] != "[shot.png is a picture; its text could not be recognised.]" {
		t.Errorf("with the app: %+v, %v", d, err)
	}
}

func TestReadImageRefusals(t *testing.T) {
	ctx := context.Background()
	_, err := Read(ctx, fixture("huge.png"), Options{Name: "big.png"})
	if err == nil || err.Error() != "big.png is 100000 × 100000 pixels, larger than the 50 megapixels Hearth looks at" {
		t.Errorf("huge: %v", err)
	}
	png, _ := os.ReadFile(fixture("tiny.png"))
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a.jpg", png, "a.jpg does not look like a JPEG image."},
		{"a.png", []byte("not an image at all"), "a.png does not look like a PNG image."},
		{"a.webp", png, "a.webp does not look like a WebP image."},
		{"a.png", png[:20], "a.png could not be read as a picture; it may be damaged or cut short"},
	}
	for _, tc := range cases {
		if _, err := Read(ctx, put(t, tc.name, tc.data), Options{}); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestMaxPixelsBoundary(t *testing.T) {
	// 10,000 x 5,000 is exactly 50 megapixels and is read; one more column is not.
	if int64(10000)*5000 > MaxPixels || int64(10001)*5000 <= MaxPixels {
		t.Errorf("MaxPixels %d is not 50 megapixels", MaxPixels)
	}
}

func TestDetectNewKinds(t *testing.T) {
	zip := []byte("PK\x03\x04")
	for _, tc := range []struct {
		name string
		head []byte
		want Kind
	}{
		{"a.xlsx", zip, KindXLSX},
		{"A.XLSX", zip, KindXLSX},
		{"a.rtf", []byte(`{\rtf1`), KindRTF},
		{"a.jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, KindJPEG},
		{"a.gif", []byte("GIF89a"), KindGIF},
		{"a.webp", tinyWebP, KindWebP},
		{"noext", []byte("\x89PNG\r\n\x1a\n"), KindPNG},
		{"noext", []byte(`{\rtf1`), KindRTF},
	} {
		if got, err := Detect(tc.name, tc.head); err != nil || got != tc.want {
			t.Errorf("Detect(%q) = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	for _, name := range []string{"a.xlsx", "a.rtf", "a.png", "a.jpg", "a.gif", "a.webp"} {
		if _, err := Detect(name, []byte("plain text")); err == nil || !strings.HasPrefix(err.Error(), name+" does not look like ") {
			t.Errorf("Detect(%q, text): %v", name, err)
		}
	}
	// An Office zip with a Word part in its first bytes is not taken for a workbook.
	if _, err := Detect("a.xlsx", append(zip, []byte("word/document.xml")...)); err == nil {
		t.Error("a.xlsx holding word/ was accepted")
	}
}
