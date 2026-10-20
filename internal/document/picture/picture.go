package picture

import (
	"context"
	"errors"
	"unicode"
	"unicode/utf8"
)

// Batch is the most pages one call to a Recogniser is asked to read.
const Batch = 10

// Recognised is the text a Recogniser found on one page.
type Recognised struct {
	Page int
	Text string
	// FromTextLayer is a page the recogniser read from the text layer itself
	// rather than by looking at it.
	FromTextLayer bool
	// Labels are what an image shows, most confident first.
	Labels []string
}

// Recogniser reads the pages that are pictures. The app provides one while it
// is open; a nil Recogniser is the command-line helper. path is the absolute
// path of the file and pages are counted from 1, at most Batch of them; an
// image is page 1. A page left out of the answer was not read.
type Recogniser interface {
	Recognise(ctx context.Context, path string, pages []int) ([]Recognised, error)
}

// ErrNoApp is a Recogniser that could not reach the app: Hearth is not open,
// and the notice says so rather than that the pages could not be recognised.
var ErrNoApp = errors.New("hearth isn't open")

// Answer is what a Recogniser made of the pages offered: the pages it
// answered, by number, and the pages it did not, split by whether the app
// was there to ask.
type Answer struct {
	Got map[int]Recognised
	// Missed were offered and not answered; Absent could not be offered
	// because the app is not open.
	Missed, Absent []int
}

// Offer offers pages to rec at most Batch at a time, in order. A batch that
// fails leaves its pages missed and the offer goes on; once the app is found
// gone, or ctx has ended, the pages not yet offered are left without asking.
// Only a page that was asked for, and only its first answer, is kept, so an
// answer cannot add or repeat a page.
func Offer(ctx context.Context, rec Recogniser, path string, pages []int) Answer {
	r := Answer{Got: map[int]Recognised{}}
	for i := 0; i < len(pages); i += Batch {
		batch := pages[i:min(i+Batch, len(pages))]
		if ctx.Err() != nil {
			r.Missed = append(r.Missed, pages[i:]...)
			break
		}
		answer, err := rec.Recognise(ctx, path, batch)
		if errors.Is(err, ErrNoApp) {
			r.Absent = append(r.Absent, pages[i:]...)
			break
		}
		asked := map[int]bool{}
		for _, n := range batch {
			asked[n] = err == nil
		}
		for _, a := range answer {
			if asked[a.Page] {
				r.Got[a.Page], asked[a.Page] = a, false
			}
		}
		for _, n := range batch {
			if _, ok := r.Got[n]; !ok {
				r.Missed = append(r.Missed, n)
			}
		}
	}
	return r
}

// IsPage is the rule for a page whose text is not text: it has fewer than 20
// characters that are not white space, or more than a tenth of its characters
// are U+FFFD or not printable (a font with no map to Unicode). An empty page
// is a picture. It judges the text as the parser returned it, before
// cleaning.
func IsPage(text string) bool {
	var solid, total, bad int
	for _, r := range text {
		total++
		switch {
		case r == utf8.RuneError || (!unicode.IsPrint(r) && !unicode.IsSpace(r)):
			bad++
			solid++
		case !unicode.IsSpace(r):
			solid++
		}
	}
	return solid < 20 || bad*10 > total
}
