package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rfizzle/hearth/internal/document/office"
)

// How a refusal names the Office kinds.
const (
	labelDocx = "a Word document"
	labelPPTX = "a PowerPoint deck"
)

// sniffZip is a file that begins as a zip does and holds no part that
// belongs to one of the other Office kinds in its first bytes. A zip's first
// entries are named in its first bytes, so a deck named .docx is often caught
// here; the part check in openOffice settles every other case.
func sniffZip(head []byte, others ...string) bool {
	if !bytes.HasPrefix(head, office.Magic) {
		return false
	}
	for _, o := range others {
		if bytes.Contains(head, []byte(o)) {
			return false
		}
	}
	return true
}

// openOffice opens f as the Office kind want: a zip whose main part is that
// kind's. label is how a refusal names the kind. A file that is a zip of
// another kind is refused in the words Detect uses for a wrong header.
func openOffice(ctx context.Context, f io.ReaderAt, size int64, name string, want office.Format, label string) (*office.Archive, error) {
	a, err := office.Open(f, size, office.Default)
	if err == nil {
		var got office.Format
		if got, err = a.Format(ctx); err == nil {
			if got != want {
				return nil, refusal(fmt.Sprintf("%s does not look like %s.", name, label))
			}
			return a, nil
		}
	}
	return nil, officeError(ctx, name, label, err)
}

// openAs is the document a read of kind starts from and the archive opened
// for it. A nil archive means the read is over: the document carries the
// notice when the time limit ended it, else err is the refusal.
func openAs(ctx context.Context, f io.ReaderAt, size int64, name string, kind Kind, want office.Format, label, slow string) (*office.Archive, Document, error) {
	doc := Document{Kind: kind}
	a, err := openOffice(ctx, f, size, name, want, label)
	if err != nil {
		doc, err = stopped(doc, slow, err)
	}
	return a, doc, err
}

// officeError is err from the office package as a sentence for the model;
// the context ending is the context's error.
func officeError(ctx context.Context, name, label string, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, office.ErrLimit):
		return fmt.Errorf("%s is larger or more deeply built inside than Hearth reads, so it was not opened", name)
	}
	return fmt.Errorf("%s could not be read as %s; it may be damaged or cut short", name, label)
}

// stopped is a read that the time limit ended, as the document with its
// notice; any other err is a refusal.
func stopped(doc Document, notice string, err error) (Document, error) {
	if errors.Is(err, context.DeadlineExceeded) {
		doc.Notices = []string{notice}
		return doc, nil
	}
	return doc, err
}

// refusePages is the refusal for pages asked of a kind that has none.
func refusePages(o Options, name, what string) error {
	if o.Pages != (Range{}) {
		return fmt.Errorf("%s is %s, which has no pages to pick; leave pages out", name, what)
	}
	return nil
}
