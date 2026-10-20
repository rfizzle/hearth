package draft

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/org/memory"
)

// The bounds every string of a reply is cut to before it is served, in
// characters. The model's reply is text from across the trust boundary and
// the owner edits the card anyway, so a long string is cut, not refused: a
// refusal for one long sentence would hide a usable draft. The numbers are
// project.Validate's for a project and the widget label's for a working style.
const (
	maxName  = 80   // a bot's or a project's name
	maxRole  = 80   // a bot's role
	maxStyle = 80   // a working style's line
	maxPart  = 4000 // a brief's part, and the job description

	dateLayout = "2006-01-02"
)

// line is s as a field that holds one line: trimmed, made one line, cut to
// n characters and trimmed again, since the cut may leave a space.
func line(s string, n int) string {
	return strings.TrimSpace(cut(memory.OneLine(strings.TrimSpace(s)), n))
}

// multi is s as a field that may span lines: trimmed and cut to n
// characters.
func multi(s string, n int) string {
	return strings.TrimSpace(cut(strings.TrimSpace(s), n))
}

// day is s when it is a day written yyyy-mm-dd, else empty: a date the model
// got wrong is left for the owner to set.
func day(s string) string {
	s = strings.TrimSpace(s)
	if _, err := time.Parse(dateLayout, s); err != nil {
		return ""
	}
	return s
}

// cut is s cut to at most n characters.
func cut(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

// read is the model's text as one T: the object from the first opening
// brace to the last closing one, so a code fence or a sentence around it does
// not matter, decoded with no field T lacks. Anything that is not exactly one
// object, an unknown field, a value of the wrong type or a second value after
// the object is ErrMalformed.
func read[T any](s string) (T, error) {
	var v T
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return v, ErrMalformed
	}
	dec := json.NewDecoder(strings.NewReader(s[start : end+1]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, ErrMalformed
	}
	if _, err := dec.Token(); err != io.EOF {
		return v, ErrMalformed
	}
	return v, nil
}
