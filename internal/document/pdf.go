package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rfizzle/hearth/internal/document/pdftext"
)

// KindPDF is a PDF, read from its text layer.
const KindPDF Kind = "pdf"

func sniffPDF(head []byte) bool { return bytes.HasPrefix(head, []byte("%PDF-")) }

// readPDF reads the pages o.Pages names, at most MaxPages of them, stopping
// at the first of: the end of the range, MaxText bytes of text, o.Want bytes
// of text, or ctx ending. A page the parser fails on is unread and the read
// goes on.
func readPDF(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	doc := Document{Kind: KindPDF}
	r, err := pdftext.Open(ctx, f, size)
	switch {
	case ctx.Err() != nil: // the minute is up before the file was open
		doc.Notices = []string{noticeTimeLimitOf(o.Limit, 0, 0, "page")}
		return doc, nil
	case errors.Is(err, pdftext.ErrPassword):
		doc.Notices = []string{noticePassword(name)}
		return doc, nil
	case err != nil:
		return doc, fmt.Errorf("%s could not be read as a PDF; it may be damaged or cut short", name)
	}
	total := r.NumPage()
	doc.PageCount = total
	if total <= 0 {
		doc.Notices = []string{noticeNoPages(name)}
		return doc, nil
	}
	src := pages{
		noun:    "page",
		heading: func(n int) string { return fmt.Sprintf("[Page %d]", n) },
		empty:   "[No text on this page.]",
		failed:  "[This page could not be read.]",
		get:     func(ctx context.Context, n, _ int) (string, error) { return r.Page(ctx, n) },
		picture: IsPicturePage,
	}
	pictures, err := readPages(ctx, &doc, total, name, o, src)
	if err != nil {
		return doc, err
	}
	if len(pictures) > 0 {
		doc.Notices = append(recognisePages(ctx, &doc, pictures, o, src), doc.Notices...)
	}
	return doc, nil
}
