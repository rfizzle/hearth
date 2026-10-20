package pdftext

import (
	"context"
	"errors"
	"io"
	"sync"

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

// slots bounds the parses that can be running, or stuck, at once. A parse
// that never returns keeps its slot until the helper exits, so a crafted
// file can pin at most this many cores however often it is read.
var (
	slotsMu sync.Mutex
	slots   = make(chan struct{}, 2)
)

// UseSlots replaces the pool with a fresh one of n slots and returns a
// function that puts the old one back. A test that leaves a parse stuck on
// purpose uses it so the stuck parse does not starve the tests after it.
func UseSlots(n int) (restore func()) {
	slotsMu.Lock()
	defer slotsMu.Unlock()
	old := slots
	slots = make(chan struct{}, n)
	return func() {
		slotsMu.Lock()
		defer slotsMu.Unlock()
		slots = old
	}
}

// run calls fn on its own goroutine, holding a slot until fn returns, and
// waits for it or for ctx to end. It returns ctx's error when no slot comes
// free in time or fn does not return in time, and ErrPage when fn panics. fn
// is left running in the second case; the caller must not use what it
// touches again. Every call into the parser goes through here.
func run(ctx context.Context, fn func() error) error {
	slotsMu.Lock()
	pool := slots
	slotsMu.Unlock()
	select {
	case pool <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		err := ErrPage // what a panic leaves
		defer func() {
			_ = recover()
			<-pool
			done <- err
		}()
		err = fn()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reader is an open PDF.
type Reader struct {
	r     *pdf.Reader
	pages int
	// parse reads one page; a test replaces it to stand in for a page that
	// never returns or one that panics.
	parse func(n int) (string, error)
}

// Open reads the file's structure from f, size bytes long, and the page
// count it claims. The empty password is tried for an encrypted file and no
// other. A parser panic is ErrDamaged; ctx ending first is ctx's error, and
// then the parse may still be running.
func Open(ctx context.Context, f io.ReaderAt, size int64) (*Reader, error) {
	var r *Reader
	err := run(ctx, func() error {
		pr, err := pdf.NewReader(f, size)
		switch {
		case errors.Is(err, pdf.ErrInvalidPassword):
			return ErrPassword
		case err != nil:
			return ErrDamaged
		}
		r = &Reader{r: pr, pages: pr.NumPage()}
		r.parse = r.pageText
		return nil
	})
	switch {
	case err == nil:
		return r, nil
	case errors.Is(err, ErrPage): // the parser panicked
		return nil, ErrDamaged
	}
	return nil, err
}

// NumPage is the page count the file claims, which a crafted file can make
// untrue.
func (r *Reader) NumPage() int { return r.pages }

// Page is the text of page n, counted from 1. It returns ctx's error when
// ctx ends first, and ErrPage when the page is missing or the parser fails
// or panics.
func (r *Reader) Page(ctx context.Context, n int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var text string
	err := run(ctx, func() (err error) {
		text, err = r.parse(n)
		if err != nil {
			err = ErrPage
		}
		return err
	})
	if err != nil {
		return "", err
	}
	return text, nil
}

func (r *Reader) pageText(n int) (string, error) {
	p := r.r.Page(n)
	if p.V.IsNull() {
		return "", ErrPage
	}
	return p.GetPlainText(nil)
}
