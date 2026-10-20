package document

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/rfizzle/hearth/internal/document/office"
)

// KindDocx is a Word file, read from its document part.
const KindDocx Kind = "docx"

func sniffDocx(head []byte) bool { return sniffZip(head, "ppt/", "xl/") }

// readDocx reads the whole of a Word file as Markdown, up to MaxText bytes.
// It has no pages; o.Want does not shorten it, as a document parses in the
// time it takes to read it.
func readDocx(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	if err := refusePages(o, name, "a Word document"); err != nil {
		return Document{Kind: KindDocx}, err
	}
	slow := noticeTimeLimitWhole(o.Limit)
	a, doc, err := openAs(ctx, f, size, name, KindDocx, office.Word, labelDocx, slow)
	if a == nil {
		return doc, err
	}
	w, err := a.Word(ctx, MaxText)
	switch {
	case errors.Is(err, office.ErrPanic):
		doc.Notices = []string{noticeUnreadFile(name)}
		return doc, nil
	case err != nil:
		return stopped(doc, slow, officeError(ctx, name, labelDocx, err))
	}
	doc.Text = clean(w.Text)
	switch {
	case strings.TrimSpace(doc.Text) == "":
		doc.Text = ""
		doc.Notices = []string{noticeEmpty(name)}
	case w.Cut:
		doc.Notices = []string{noticeTextStop()}
	}
	if w.Images || w.Footnotes {
		doc.Notices = append(doc.Notices, noticeLeftOut(w.Images, w.Footnotes))
	}
	return doc, nil
}
