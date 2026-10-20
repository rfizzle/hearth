package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rfizzle/hearth/internal/document/rtf"
)

// KindRTF is a Rich Text Format file, read as its text.
const KindRTF Kind = "rtf"

// readRTF reads the file's text with its formatting dropped, at most MaxText
// bytes of it.
func readRTF(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error) {
	doc := Document{Kind: KindRTF}
	if err := refusePages(o, name, "an RTF file"); err != nil {
		return doc, err
	}
	text, cut, err := rtf.Strip(ctx, io.NewSectionReader(f, 0, size), MaxText)
	if ctx.Err() != nil {
		doc.Notices = []string{noticeTimeLimitWhole(o.Limit)}
		return doc, nil
	}
	if errors.Is(err, rtf.ErrTooDeep) {
		return doc, fmt.Errorf("%s is larger or more deeply built inside than Hearth reads, so it was not opened", name)
	}
	if err != nil {
		return doc, refusal(fmt.Sprintf("%s does not look like an RTF file.", name))
	}
	doc.Text = clean(text)
	doc.Header = "text without its formatting"
	if cut {
		doc.Notices = append(doc.Notices, noticeTextStop())
	}
	if doc.Text == "" {
		doc.Notices = append(doc.Notices, noticeEmpty(name))
	}
	return doc, nil
}

// sniffRTF is a file that begins with the RTF group.
func sniffRTF(head []byte) bool { return bytes.HasPrefix(head, []byte(`{\rtf`)) }
