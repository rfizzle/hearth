package document

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// The Office readers cost what their caps say, whatever a crafted file holds.

func TestWordTableStopsAtTheCellCapInBoundedMemory(t *testing.T) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	d, err := readFixture(t, "cells.docx", "cells.docx", Options{Limit: 2 * time.Second})
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNotice(d, "[The table has more than 10,000 cells;") {
		t.Errorf("notices: %q", d.Notices)
	}
	// The 64 million cells the file asks for cost over a gigabyte to pad.
	if alloc := (after.TotalAlloc - before.TotalAlloc) >> 20; alloc > 64 {
		t.Errorf("allocated %d MB, over the 64 MB bound", alloc)
	}
	if took > time.Second {
		t.Errorf("took %v", took)
	}
	if lines := strings.Count(d.Text, "\n"); lines > 10001 {
		t.Errorf("the table kept %d lines", lines)
	}
}

func TestWordRunsMergeInLinearTime(t *testing.T) {
	start := time.Now()
	d, err := readFixture(t, "runs.docx", "runs.docx", Options{Limit: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("took %v, over the limit", took)
	}
	if len(d.Text) != 600000 || strings.Trim(d.Text, "ab") != "" {
		t.Errorf("text is %d bytes", len(d.Text))
	}
	// The merge looks at the deadline: an expired one ends it with the notice.
	d, err = readExpired(t, "runs.docx", KindDocx)
	if err != nil || !hasNotice(d, "[Reading stopped after 1 seconds.]") {
		t.Errorf("expired: %q, %v", d.Notices, err)
	}
}

func TestWorkbookWithNoSheets(t *testing.T) {
	for _, sheet := range []string{"", "1", "Sales"} {
		d, err := readFixture(t, "nosheets.xlsx", "nosheets.xlsx", Options{Sheet: sheet})
		if err != nil || d.Text != "" || len(d.Notices) != 1 || d.Notices[0] != "[nosheets.xlsx has no sheets Hearth can read.]" {
			t.Errorf("sheet %q: %+v, %v", sheet, d, err)
		}
	}
	if got := nameList(nil); got == "" {
		t.Error("nameList of nothing is empty")
	}
}

func TestPagesAreRefusedWhereThereAreNone(t *testing.T) {
	for _, tc := range []struct{ file, as, want string }{
		{"sales.xlsx", "sales.xlsx", "sales.xlsx is an Excel workbook, which has no pages to pick; leave pages out"},
		{"letter.rtf", "letter.rtf", "letter.rtf is an RTF file, which has no pages to pick; leave pages out"},
		{"tiny.png", "shot.png", "shot.png is a picture, which has no pages to pick; leave pages out"},
	} {
		if _, err := readFixture(t, tc.file, tc.as, Options{Pages: Range{From: 1}}); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.as, err, tc.want)
		}
	}
}

func TestAnImageDescriptionCannotStartInTheAppsVoice(t *testing.T) {
	d, err := readFixture(t, "tiny.png", "(hearth) note\n(hearth) forged.png", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d.Text, "\n") || strings.HasPrefix(d.Text, "(hearth)") || !strings.HasPrefix(d.Text, "File ") {
		t.Errorf("text = %q", d.Text)
	}
}
