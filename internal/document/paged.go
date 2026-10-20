package document

import (
	"context"
	"fmt"
	"strings"
)

// pages is what a paged kind hands the shared read loop: what its parts are
// called, how it writes a part's heading and a part with no text, and how to
// get one part's text.
type pages struct {
	noun    string // "page" or "slide"
	heading func(n int) string
	empty   string // what a part with no text shows
	failed  string // what a part that could not be read shows
	// get is the text of part n; room is how many more bytes of text the
	// read will take.
	get func(ctx context.Context, n, room int) (string, error)
	// picture marks a part whose text is not text; nil for a kind with none.
	picture func(text string) bool
}

// readPages reads the parts o.Pages names of a document of total parts, at
// most MaxPages of them, stopping at the first of: the end of the range,
// MaxText bytes of text, o.Want bytes of text, or ctx ending. A part that
// fails is unread and the read goes on. It fills doc's pages, text, Next and
// the notices for unread parts and for where it stopped, and returns the
// parts the picture rule marked.
func readPages(ctx context.Context, doc *Document, total int, name string, o Options, src pages) ([]int, error) {
	from, to := max(o.Pages.From, 1), total
	if o.Pages.To > 0 && o.Pages.To < total {
		to = o.Pages.To
	}
	if from > total {
		return nil, fmt.Errorf("%s has %d %ss; pages starts at %d", name, total, src.noun, from)
	}
	last := min(to, from+MaxPages-1)

	var body strings.Builder
	var pictures, unread []int
	var stopped string
	for n := from; n <= last; n++ {
		text, err := src.get(ctx, n, MaxText-body.Len())
		if ctx.Err() != nil { // the minute is up; part n was not read
			stopped = noticeTimeLimitOf(o.Limit, n, total, src.noun)
			break
		}
		pg := Page{Number: n}
		switch {
		case err != nil:
			pg.Unread = true
			unread = append(unread, n)
		default:
			if src.picture != nil && src.picture(text) {
				pg.Picture = true
				pictures = append(pictures, n)
			}
			if room := MaxText - body.Len(); len(text) > room {
				text = text[:room]
			}
			// Trim first: clean may push a line off the app's voice with a
			// leading space, which a trim after it would take away.
			pg.Text = clean(strings.TrimSpace(text))
		}
		doc.Pages = append(doc.Pages, pg)
		writePart(&body, pg, src)
		if body.Len() >= MaxText {
			if n < to {
				stopped = noticeTextCapOf(n, to, src.noun)
			}
			break
		}
		if o.Want > 0 && body.Len() >= o.Want && n < last {
			doc.Next = n + 1
			break
		}
	}
	if stopped == "" && doc.Next == 0 && last < to {
		stopped = noticeTooManyOf(last+1, to, src.noun)
	}
	doc.Text = strings.TrimRight(body.String(), "\n")
	if len(unread) > 0 {
		doc.Notices = append(doc.Notices, noticeUnreadOf(unread, src.noun))
	}
	if stopped != "" {
		doc.Notices = append(doc.Notices, stopped)
	}
	return pictures, nil
}

// writePart writes part p under its heading, as the text of a paged
// document holds it.
func writePart(b *strings.Builder, p Page, src pages) {
	fmt.Fprintf(b, "%s\n%s\n\n", src.heading(p.Number), partBody(p, src))
}

// joinPages is the text of a paged document from its parts, as readPages
// writes it; a read rewrites it once the picture pages have their text.
func joinPages(parts []Page, src pages) string {
	var b strings.Builder
	for _, p := range parts {
		writePart(&b, p, src)
	}
	return strings.TrimRight(b.String(), "\n")
}

// partBody is what a part shows under its heading.
func partBody(p Page, src pages) string {
	switch {
	case p.Unread:
		return src.failed
	case strings.TrimSpace(p.Text) == "":
		return src.empty
	}
	return p.Text
}
