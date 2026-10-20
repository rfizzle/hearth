package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
	r, err := pdftext.Open(f, size)
	switch {
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
	from, to := max(o.Pages.From, 1), total
	if o.Pages.To > 0 && o.Pages.To < total {
		to = o.Pages.To
	}
	if from > total {
		return doc, fmt.Errorf("%s has %d pages; pages starts at %d", name, total, from)
	}
	last := min(to, from+MaxPages-1)

	var body strings.Builder
	var pictures, unread []int
	var stopped string
	for n := from; n <= last; n++ {
		text, err := r.Page(ctx, n)
		if ctx.Err() != nil { // the minute is up; page n was not read
			stopped = noticeTimeLimit(o.Limit, n, total)
			break
		}
		pg := Page{Number: n}
		switch {
		case err != nil:
			pg.Unread = true
			unread = append(unread, n)
		case IsPicturePage(text):
			pg.Picture = true
			pictures = append(pictures, n)
			fallthrough
		default:
			if room := MaxText - body.Len(); len(text) > room {
				text = text[:room]
			}
			// Trim first: clean may push a line off the app's voice with a
			// leading space, which a trim after it would take away.
			pg.Text = clean(strings.TrimSpace(text))
		}
		doc.Pages = append(doc.Pages, pg)
		fmt.Fprintf(&body, "[Page %d]\n%s\n\n", n, pageBody(pg))
		if body.Len() >= MaxText {
			if n < to {
				stopped = noticeTextCap(n, to)
			}
			break
		}
		if o.Want > 0 && body.Len() >= o.Want && n < last {
			doc.Next = n + 1
			break
		}
	}
	if stopped == "" && doc.Next == 0 && last < to {
		stopped = noticeTooManyPages(last+1, to)
	}
	doc.Text = strings.TrimRight(body.String(), "\n")
	if len(pictures) > 0 {
		doc.Notices = append(doc.Notices, noticePictures(pictures))
	}
	if len(unread) > 0 {
		doc.Notices = append(doc.Notices, noticeUnread(unread))
	}
	if stopped != "" {
		doc.Notices = append(doc.Notices, stopped)
	}
	return doc, nil
}

// pageBody is what a page shows under its marker.
func pageBody(p Page) string {
	switch {
	case p.Unread:
		return "[This page could not be read.]"
	case strings.TrimSpace(p.Text) == "":
		return "[No text on this page.]"
	}
	return p.Text
}
