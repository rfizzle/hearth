package document

import (
	"context"
	"slices"
	"strings"

	"github.com/rfizzle/hearth/internal/document/picture"
)

// maxLabels is the most labels an image's description lists.
const maxLabels = 5

// recognisePages offers the picture pages of doc to o.Recogniser, puts the
// text it answers into those pages, cleaned and within what is left of
// MaxText, rewrites doc.Text from the pages, and returns the notices that
// say how each picture page was read or why it was not. Without a
// Recogniser it is the notice that Hearth isn't open.
func recognisePages(ctx context.Context, doc *Document, pictures []int, o Options, src pages) []string {
	if o.Recogniser == nil {
		return []string{picture.Unread(pictures, false)}
	}
	r := picture.Offer(ctx, o.Recogniser, o.path, pictures)
	room := MaxText - len(doc.Text)
	var layer, seen, empty []int
	for i, p := range doc.Pages {
		a, ok := r.Got[p.Number]
		if !ok {
			continue
		}
		text := clean(strings.TrimSpace(cut(a.Text, max(room, 0))))
		if strings.TrimSpace(text) == "" {
			empty = append(empty, p.Number)
			continue
		}
		room -= len(text)
		doc.Pages[i].Text = text
		if a.FromTextLayer {
			layer = append(layer, p.Number)
		} else {
			seen = append(seen, p.Number)
		}
	}
	doc.Text = joinPages(doc.Pages, src)
	var notices []string
	if len(layer) > 0 {
		notices = append(notices, picture.TextLayer(layer))
	}
	if len(seen) > 0 {
		notices = append(notices, picture.Seen(seen))
	}
	if missed := append(r.Missed, empty...); len(missed) > 0 {
		slices.Sort(missed)
		notices = append(notices, picture.Unread(missed, true))
	}
	if len(r.Absent) > 0 {
		notices = append(notices, picture.Unread(r.Absent, false))
	}
	return notices
}

// recogniseImage asks o.Recogniser for the text in the image at o.path and
// what it shows. It returns the lines to add under the image's description,
// cleaned and capped, and the notice for them.
func recogniseImage(ctx context.Context, name string, o Options) (string, string) {
	if o.Recogniser == nil {
		return "", picture.Image(name, false, false, false)
	}
	r := picture.Offer(ctx, o.Recogniser, o.path, []int{1})
	a, ok := r.Got[1]
	if len(r.Absent) > 0 || !ok {
		return "", picture.Image(name, len(r.Absent) == 0, false, false)
	}
	var b strings.Builder
	text := clean(strings.TrimSpace(cut(a.Text, MaxText)))
	found := strings.TrimSpace(text) != ""
	if found {
		b.WriteString("\n\nText in the image:\n" + text)
	}
	var labels []string
	for _, l := range a.Labels {
		if l = label(l); l != "" && len(labels) < maxLabels {
			labels = append(labels, l)
		}
	}
	if len(labels) > 0 {
		b.WriteString("\n\nLabels: " + strings.Join(labels, ", ") + ".")
	}
	return b.String(), picture.Image(name, true, true, found)
}

// cut is s at most n bytes long, without a rune cut in two.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
