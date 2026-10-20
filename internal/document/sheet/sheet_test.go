package sheet

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func read(t *testing.T, b []byte, sel string) (Result, error) {
	t.Helper()
	return Read(context.Background(), bytes.NewReader(b), int64(len(b)), sel)
}

// book is a workbook of n sheets called S1..Sn, each with one cell.
func book(t *testing.T, n int) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "S1"); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= n; i++ {
		if _, err := f.NewSheet(fmt.Sprintf("S%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= n; i++ {
		_ = f.SetCellValue(fmt.Sprintf("S%d", i), "A1", fmt.Sprintf("cell %d", i))
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// zipOf is a zip of the named entries, each stored with the bytes given.
func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// zipRaw is a zip whose one entry has a header that says claimed bytes and
// holds data, already deflated, whatever that decompresses to.
func zipRaw(t *testing.T, name string, claimed uint64, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.CreateRaw(&zip.FileHeader{Name: name, Method: zip.Deflate, UncompressedSize64: claimed, CompressedSize64: uint64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(data)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func deflated(t *testing.T, n int) []byte {
	t.Helper()
	var b bytes.Buffer
	w, _ := flate.NewWriter(&b, flate.BestCompression)
	_, _ = w.Write(make([]byte, n))
	w.Close()
	return b.Bytes()
}

func TestReadShowsDatesAndNumbersAsExcelDoes(t *testing.T) {
	res, err := read(t, fixture(t, "sales.xlsx"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Names) != 2 || res.Names[0] != "Sales" || len(res.Sheets) != 2 || len(res.Left) != 0 {
		t.Fatalf("result %+v, want two sheets Sales and Notes", res)
	}
	s := res.Sheets[0]
	for _, want := range []string{"| Day | Region | Amount | Doubled |", "| --- | --- | --- | --- |", "| 2024-01-31 | North \\| East | 1,234.50 |", "| 2024-02-29 | South | 99.00 |"} {
		if !strings.Contains(s.Table, want) {
			t.Errorf("table lacks %q:\n%s", want, s.Table)
		}
	}
	if s.Rows != 3 || s.Cols != 4 || s.Cut || !s.Exact {
		t.Errorf("sheet %+v, want 3 rows by 4 columns, not cut", s)
	}
	if !strings.Contains(res.Sheets[1].Table, "Figures are before tax.") {
		t.Errorf("Notes table %q", res.Sheets[1].Table)
	}
}

func TestReadCutsASheetAtTheCellCap(t *testing.T) {
	res, err := read(t, fixture(t, "wide.xlsx"), "")
	if err != nil {
		t.Fatal(err)
	}
	s := res.Sheets[0]
	if !s.Cut || s.Rows != 120 || s.Cols != 100 || !s.Exact || s.Shown != 10_000 {
		t.Errorf("sheet cut %v rows %d cols %d shown %d, want cut, 120 by 100, 10,000 shown", s.Cut, s.Rows, s.Cols, s.Shown)
	}
	if lines := strings.Count(s.Table, "\n") + 1; lines != 100+1 {
		t.Errorf("table has %d lines, want a header, a separator and 99 rows", lines)
	}
}

func TestReadStopsAtTwentySheets(t *testing.T) {
	res, err := read(t, book(t, 23), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) != MaxSheets || len(res.Left) != 3 || res.Left[0] != "S21" || len(res.Names) != 23 {
		t.Errorf("read %d sheets, left %v of %d; want 20 read and S21..S23 left", len(res.Sheets), res.Left, len(res.Names))
	}
}

func TestSheetArgument(t *testing.T) {
	b := book(t, 23)
	for _, tc := range []struct{ sel, want string }{
		{"S22", "S22"}, {"s22", "S22"}, {"22", "S22"}, {" 3 ", "S3"}, {"1", "S1"},
	} {
		res, err := read(t, b, tc.sel)
		if err != nil || len(res.Sheets) != 1 || res.Sheets[0].Name != tc.want || len(res.Left) != 0 {
			t.Errorf("sheet %q: %+v, %v; want only %s", tc.sel, res, err, tc.want)
		}
	}
	for _, sel := range []string{"nope", "0", "24", "-1"} {
		_, err := read(t, b, sel)
		var none *NoSheetError
		if !errors.As(err, &none) || len(none.Names) != 23 {
			t.Errorf("sheet %q: err %v, want NoSheetError naming all 23", sel, err)
		}
	}
}

func TestANameBeatsANumber(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "2")
	_, _ = f.NewSheet("1")
	var b bytes.Buffer
	_ = f.Write(&b)
	res, err := read(t, b.Bytes(), "1")
	if err != nil || res.Sheets[0].Name != "1" {
		t.Errorf("sheet \"1\": %+v, %v; want the sheet called 1, not the first", res, err)
	}
}

func TestZipRefusals(t *testing.T) {
	tooMany := map[string]string{"xl/workbook.xml": "<workbook/>"}
	for i := 0; i < MaxEntries; i++ {
		tooMany[fmt.Sprintf("e%d", i)] = ""
	}
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"not a zip", []byte("plain words, not a zip at all"), ErrDamaged},
		{"empty", nil, ErrDamaged},
		{"old binary or encrypted container", append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 64)...), ErrEncrypted},
		{"no workbook part", zipOf(t, map[string]string{"word/document.xml": "<w/>"}), ErrNotWorkbook},
		{"more than the entry cap", zipOf(t, tooMany), ErrLimit},
		{"an entry whose header claims more than the entry cap", zipRaw(t, "xl/workbook.xml", MaxEntry+1, deflated(t, 10)), ErrLimit},
		{"entries whose headers claim more than the total", func() []byte {
			var b bytes.Buffer
			w := zip.NewWriter(&b)
			for i := 0; i < 5; i++ {
				name := fmt.Sprintf("xl/part%d.xml", i)
				if i == 0 {
					name = "xl/workbook.xml"
				}
				f, _ := w.CreateRaw(&zip.FileHeader{Name: name, Method: zip.Deflate, UncompressedSize64: 60 << 20, CompressedSize64: 2})
				_, _ = f.Write([]byte{3, 0})
			}
			w.Close()
			return b.Bytes()
		}(), ErrLimit},
		{"a header that understates what the entry yields", zipRaw(t, "xl/workbook.xml", 10, deflated(t, 1<<20)), ErrDamaged},
		{"a header that overstates what the entry yields", zipRaw(t, "xl/workbook.xml", 1<<20, deflated(t, 10)), ErrDamaged},
	}
	for _, tc := range cases {
		if _, err := read(t, tc.data, ""); !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestAnEntryThatInflatesPastTheCapIsRefused(t *testing.T) {
	// The header is honest here: 65 MB of zeros deflate to about 64 KB.
	data := zipRaw(t, "xl/workbook.xml", MaxEntry+1<<20, deflated(t, MaxEntry+1<<20))
	if _, err := read(t, data, ""); !errors.Is(err, ErrLimit) {
		t.Errorf("err %v, want ErrLimit", err)
	}
}

func TestOptionsKeepTheLibraryInMemory(t *testing.T) {
	o := openOptions()
	if o.UnzipSizeLimit != MaxTotal || o.UnzipXMLSizeLimit != MaxTotal {
		t.Errorf("limits %d and %d, want both %d so no part is spooled to disk", o.UnzipSizeLimit, o.UnzipXMLSizeLimit, MaxTotal)
	}
	if fi, err := os.Stat(o.TmpDir); err == nil && fi.IsDir() {
		t.Errorf("TmpDir %q is a folder; a temporary file could be written", o.TmpDir)
	}
}

func TestReadAfterTheDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	b := fixture(t, "sales.xlsx")
	res, err := Read(ctx, bytes.NewReader(b), int64(len(b)), "")
	if !errors.Is(err, context.DeadlineExceeded) || res.Stopped < 1 || len(res.Sheets) != 0 {
		t.Errorf("res %+v, err %v; want the deadline, nothing read", res, err)
	}
}

func TestReadWithNoFreeSlotWaitsForTheDeadline(t *testing.T) {
	defer UseSlots(0)()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	b := fixture(t, "sales.xlsx")
	res, err := Read(ctx, bytes.NewReader(b), int64(len(b)), "")
	if !errors.Is(err, context.DeadlineExceeded) || res.Stopped != 1 {
		t.Errorf("res %+v, err %v; want the deadline at once", res, err)
	}
}

func TestADeadlineInTheMiddleOfASheetDropsThatSheet(t *testing.T) {
	f, err := excelize.OpenReader(bytes.NewReader(fixture(t, "wide.xlsx")), openOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readSheet(ctx, f, "Sheet1"); !errors.Is(err, context.Canceled) {
		t.Errorf("err %v, want the sheet dropped with the context's error", err)
	}
}

func TestAPanicInTheLibraryLeavesTheSheetUnread(t *testing.T) {
	s, err := readSheet(context.Background(), nil, "Sheet1") // a nil workbook panics in the library
	if err != nil || !s.Unread || s.Name != "Sheet1" {
		t.Errorf("sheet %+v, err %v; want it unread, no error", s, err)
	}
}

func TestCellsAreCutAtTheCap(t *testing.T) {
	got, cut := cell("short")
	if got != "short" || cut {
		t.Errorf("cell = %q, %v", got, cut)
	}
	long := strings.Repeat("é", MaxCellRunes+5)
	got, cut = cell(long)
	if !cut || len([]rune(got)) != MaxCellRunes+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("long cell: %d runes, cut %v", len([]rune(got)), cut)
	}
}

func TestAnEmptyWorkbookSheetHasNoTable(t *testing.T) {
	res, err := read(t, book(t, 1), "")
	if err != nil || len(res.Sheets) != 1 || res.Sheets[0].Table == "" {
		t.Fatalf("one cell: %+v, %v", res, err)
	}
	f := excelize.NewFile()
	defer f.Close()
	var b bytes.Buffer
	_ = f.Write(&b)
	res, err = read(t, b.Bytes(), "")
	if err != nil || len(res.Sheets) != 1 || res.Sheets[0].Table != "" || res.Sheets[0].Rows != 0 {
		t.Errorf("empty sheet: %+v, %v", res, err)
	}
}
