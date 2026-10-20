package document

import (
	"context"
	"fmt"
	"io"

	"github.com/rfizzle/hearth/internal/document/office"
)

// KindPPTX is a PowerPoint deck, read slide by slide.
const KindPPTX Kind = "pptx"

func sniffPPTX(head []byte) bool { return sniffZip(head, "word/", "xl/") }

// readPPTX reads the slides o.Pages names, at most MaxPages of them, each
// under a "## Slide n" heading, stopping at the first of: the end of the
// range, MaxText bytes of text, o.Want bytes of text, or ctx ending. A slide
// that fails to parse is unread and the read goes on. Speaker notes are not
// read.
func readPPTX(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	slow := noticeTimeLimitOf(o.Limit, 0, 0, "slide")
	a, doc, err := openAs(ctx, f, size, name, KindPPTX, office.PowerPoint, labelPPTX, slow)
	if a == nil {
		return doc, err
	}
	deck, err := a.Deck(ctx)
	if err != nil {
		return stopped(doc, slow, officeError(ctx, name, labelPPTX, err))
	}
	total := deck.Len()
	doc.PageCount = total
	if total == 0 {
		doc.Notices = []string{noticeNoPages(name)}
		return doc, nil
	}
	_, err = readPages(ctx, &doc, total, name, o, pages{
		noun:    "slide",
		heading: func(n int) string { return fmt.Sprintf("## Slide %d", n) },
		empty:   "[No text on this slide.]",
		failed:  "[This slide could not be read.]",
		get:     deck.Slide,
	})
	return doc, err
}
