package document

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rfizzle/hearth/internal/document/office"
)

// KindCSV is a comma, semicolon or tab separated file, read as one table.
const KindCSV Kind = "csv"

// sniffCSV is a file with no NUL byte in its first bytes, so a spreadsheet or
// a UTF-16 file named .csv is refused rather than read as noise.
func sniffCSV(head []byte) bool { return !bytes.Contains(head, []byte{0}) }

// delimiter is a tab for a .tsv file; for any other it is whichever of the
// comma and the semicolon is more frequent on the first line, the comma on a
// tie.
func delimiter(name string, first []byte) rune {
	if strings.EqualFold(filepath.Ext(name), ".tsv") {
		return '\t'
	}
	if line, _, _ := bytes.Cut(first, []byte("\n")); bytes.Count(line, []byte(";")) > bytes.Count(line, []byte(",")) {
		return ';'
	}
	return ','
}

// readCSV writes the file as a Markdown table, its first row the header,
// stopping at the first of: the end of the file, office.MaxCells cells,
// MaxText bytes of text, or ctx ending. A row longer than the cell cap is
// cut to it. It has no pages and o.Want does not shorten it.
func readCSV(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	doc := Document{Kind: KindCSV}
	if err := refusePages(o, name, "a table"); err != nil {
		return doc, err
	}
	br := bufio.NewReaderSize(io.NewSectionReader(f, 0, size), 64<<10)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte("\xef\xbb\xbf")) {
		_, _ = br.Discard(3)
	}
	first, _ := br.Peek(4 << 10)
	r := csv.NewReader(br)
	r.Comma = delimiter(name, first)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	var rows [][]string
	var cells, bytesIn int
	var stop string
	for {
		if ctx.Err() != nil {
			stop = noticeTimeLimitWhole(o.Limit)
			break
		}
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(rows) == 0 {
				return doc, fmt.Errorf("%s could not be read as a CSV file; it may be damaged", name)
			}
			stop = noticeUnreadFile(name)
			break
		}
		if len(rec) > office.MaxCells {
			rec = rec[:office.MaxCells]
		}
		if cells+len(rec) > office.MaxCells {
			stop = noticeCellCap(office.MaxCells)
			break
		}
		for _, c := range rec {
			bytesIn += len(c) + 3
		}
		if bytesIn > MaxText {
			stop = noticeTextStop()
			break
		}
		rows = append(rows, rec)
		cells += len(rec)
	}
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return doc, ctx.Err()
	}
	doc.Text = clean(office.Cut(office.Table(rows), MaxText))
	if doc.Text == "" && stop == "" {
		stop = noticeEmpty(name)
	}
	if stop != "" {
		doc.Notices = []string{stop}
	}
	return doc, nil
}
