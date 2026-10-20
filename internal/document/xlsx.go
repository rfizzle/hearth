package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rfizzle/hearth/internal/document/sheet"
)

// KindXLSX is an Excel workbook, read as one table per sheet.
const KindXLSX Kind = "xlsx"

const labelXLSX = "an Excel workbook"

// readXLSX reads the workbook's sheets, the one o.Sheet names or the first
// sheet.MaxSheets of them, each as a heading and a Markdown table. Text stops
// at MaxText bytes at the end of a line.
func readXLSX(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	doc := Document{Kind: KindXLSX}
	if err := refusePages(o, name, labelXLSX); err != nil {
		return doc, err
	}
	res, err := sheet.Read(ctx, f, size, o.Sheet)
	var none *sheet.NoSheetError
	switch {
	case errors.As(err, &none) && len(none.Names) == 0: // nothing to name: the same as a workbook read without a sheet
		doc.Notices = []string{noticeNoSheets(name)}
		return doc, nil
	case errors.As(err, &none):
		return doc, fmt.Errorf("%s has no sheet called %q; its sheets are %s", name, oneLine(o.Sheet), nameList(none.Names))
	case err != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()):
		// the minute is up: what was finished is read, and a notice says where
	case errors.Is(err, sheet.ErrNotWorkbook):
		return doc, refusal(fmt.Sprintf("%s does not look like %s.", name, labelXLSX))
	case errors.Is(err, sheet.ErrEncrypted):
		return doc, refusal(fmt.Sprintf("%s is password-protected or in an old Excel format, so Hearth cannot read it. Ask the owner for a copy saved as .xlsx without the password.", name))
	case errors.Is(err, sheet.ErrLimit):
		return doc, fmt.Errorf("%s is larger or more deeply built inside than Hearth reads, so it was not opened", name)
	case err != nil:
		return doc, fmt.Errorf("%s could not be read as %s; it may be damaged or cut short", name, labelXLSX)
	}

	var body strings.Builder
	var notices, unread []string
	cellsCut := false
	capped := ""
	for _, s := range res.Sheets {
		heading := "## Sheet: " + label(s.Name)
		switch {
		case s.Unread:
			unread = append(unread, s.Name)
			fmt.Fprintf(&body, "%s\n\n[This sheet could not be read.]\n\n", heading)
			continue
		case s.Table == "":
			fmt.Fprintf(&body, "%s (0 rows × 0 columns)\n\n[No cells in this sheet.]\n\n", heading)
			continue
		}
		fmt.Fprintf(&body, "%s (%s × %s)\n\n%s\n\n", heading, plural(s.Rows, "row"), plural(s.Cols, "column"), s.Table)
		if s.Cut {
			notices = append(notices, noticeSheetCut(s.Name, s.Rows*s.Cols, s.Exact))
		}
		cellsCut = cellsCut || s.CellsCut
		if body.Len() > MaxText {
			capped = s.Name
			break
		}
	}
	text := strings.TrimRight(body.String(), "\n")
	if len(text) > MaxText {
		text = text[:max(strings.LastIndex(text[:MaxText], "\n"), 0)]
		notices = append(notices, noticeSheetTextCap(capped))
	}
	doc.Text = clean(text)
	if cellsCut {
		notices = append(notices, noticeCellsCut(sheet.MaxCellRunes))
	}
	for _, n := range unread {
		notices = append(notices, noticeSheetUnread(n))
	}
	if len(res.Left) > 0 {
		notices = append(notices, noticeSheetsLeft(res.Left))
	}
	switch {
	case res.Stopped > 0 && len(res.Names) == 0:
		notices = append(notices, noticeTimeLimitOf(o.Limit, 0, 0, "sheet"))
	case res.Stopped > 0:
		notices = append(notices, noticeTimeLimitOf(o.Limit, res.Stopped, len(res.Names), "sheet"))
	case len(res.Names) == 0:
		notices = append(notices, noticeNoSheets(name))
	}
	doc.Notices = notices
	if len(res.Names) > 0 {
		doc.Header = plural(len(res.Names), "sheet") + ", read as tables"
		if o.Sheet != "" && len(res.Sheets) == 1 {
			doc.Header = fmt.Sprintf(`%s, showing sheet "%s"`, plural(len(res.Names), "sheet"), label(res.Sheets[0].Name))
		}
	}
	return doc, nil
}

// plural is "1 sheet" or "3 sheets".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// oleHead begins the container of a password-protected workbook, which the
// reader names in its refusal rather than Detect calling it the wrong kind.
var oleHead = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// sniffXLSX is a file that begins as a zip does and does not name a Word or
// PowerPoint part in its first bytes, or begins as the container of a
// protected workbook. The workbook part itself is checked by the reader,
// which reads the zip's directory.
func sniffXLSX(head []byte) bool {
	if bytes.HasPrefix(head, oleHead) {
		return true
	}
	if !bytes.HasPrefix(head, []byte("PK\x03\x04")) {
		return false
	}
	return !bytes.Contains(head, []byte("word/")) && !bytes.Contains(head, []byte("ppt/"))
}
