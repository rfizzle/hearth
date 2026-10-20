package pdftext

import (
	"context"
	"errors"
	"io"

	"github.com/ledongthuc/pdf"
)

var (
	// ErrPassword is a PDF that needs a password to open.
	ErrPassword = errors.New("the PDF is password-protected")
	// ErrDamaged is a file that is not a PDF the parser can open: cut short,
	// malformed, or using an encryption it does not know.
	ErrDamaged = errors.New("the PDF is damaged or not supported")
	// ErrPage is a page whose text could not be read.
	ErrPage = errors.New("the page could not be read")
)

// Reader is an open PDF.
type Reader struct {
	r *pdf.Reader
	// parse reads one page; a test replaces it to stand in for a page that
	// never returns or one that panics.
	parse func(n int) (string, error)
}

// Open reads the file's structure from f, size bytes long. The empty
// password is tried for an encrypted file and no other. A parser panic is
// ErrDamaged.
func Open(f io.ReaderAt, size int64) (r *Reader, err error) {
	defer func() {
		if recover() != nil {
			r, err = nil, ErrDamaged
		}
	}()
	pr, err := pdf.NewReader(f, size)
	switch {
	case errors.Is(err, pdf.ErrInvalidPassword):
		return nil, ErrPassword
	case err != nil:
		return nil, ErrDamaged
	}
	r = &Reader{r: pr}
	r.parse = r.pageText
	return r, nil
}

// NumPage is the page count the file claims, which a crafted file can make
// untrue; 0 when it cannot be read.
func (r *Reader) NumPage() (n int) {
	defer func() {
		if recover() != nil {
			n = 0
		}
	}()
	return r.r.NumPage()
}

// Page is the text of page n, counted from 1. It returns ctx's error when
// ctx ends first, and ErrPage when the page is missing or the parser fails
// or panics.
func (r *Reader) Page(ctx context.Context, n int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		res := result{err: ErrPage}
		defer func() {
			_ = recover() // a panic leaves res as ErrPage
			done <- res
		}()
		text, err := r.parse(n)
		if err == nil {
			res = result{text: text}
		}
	}()
	select {
	case res := <-done:
		return res.text, res.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *Reader) pageText(n int) (string, error) {
	p := r.r.Page(n)
	if p.V.IsNull() {
		return "", ErrPage
	}
	return p.GetPlainText(nil)
}
