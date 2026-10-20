package paper

import (
	"errors"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// ErrNotYet is what Render returns until a shaped deliverable can be drawn as
// a PDF. A caller writes the files it can and says the PDF is not made.
var ErrNotYet = errors.New("a PDF of a shaped deliverable is not made yet")

// Render will draw a shaped deliverable as a PDF in the print style; it
// returns ErrNotYet and nothing else for now.
func Render(deliverable.Spec, Options) ([]byte, Report, error) {
	return nil, Report{}, ErrNotYet
}
