package concern

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org/memory"
)

const (
	// Max is the most concerns a draft carries.
	Max = 3
	// MaxConcern is the longest concern, in characters.
	MaxConcern = 200

	// maxShort is the bound of a one-line field (a name, a role, a working
	// style) and maxLong of a brief part or a job description; they are
	// project.Validate's and the drafters' bounds.
	maxShort = 80
	maxLong  = 4000

	dateLayout = "2006-01-02"
)

// Concern is one thought about one field of a draft.
type Concern struct {
	Field      string // the field's name in the request it is about
	Concern    string // one plain sentence, at most MaxConcern characters
	Suggestion string // a replacement for the field, or empty when there is none
}

// bounds holds every field a concern may be about, with the longest its
// suggestion may be. A field not here is unknown.
var bounds = map[string]int{
	"name": maxShort, "role": maxShort, "working_style": maxShort,
	"goal": maxLong, "context": maxLong, "done_looks_like": maxLong, "done_when": maxLong,
	"job_description": maxLong, "target_date": len(dateLayout),
}

// oneLine are the fields that hold a single line; a suggestion for any other
// may keep its line breaks.
var oneLine = map[string]bool{
	"name": true, "role": true, "working_style": true, "done_when": true, "target_date": true,
}

// Cap is the concerns to serve: the rules' first, in the order given, then
// the model's in the slots that remain, at most Max in all. A model concern
// is dropped when its field is unknown, when its field already has a concern
// from a rule or from an earlier model concern, or when its words are empty
// or, after being made one line, over MaxConcern characters. Its suggestion
// is made one line for a one-line field, stripped of control characters, and
// cut to the field's bound; a suggestion for target_date that is not a day is
// removed.
func Cap(rules, model []Concern) []Concern {
	var out []Concern
	taken := map[string]bool{}
	for _, r := range rules {
		taken[r.Field] = true
		if len(out) < Max {
			out = append(out, r)
		}
	}
	for _, m := range model {
		if len(out) >= Max {
			break
		}
		bound, known := bounds[m.Field]
		if !known || taken[m.Field] {
			continue
		}
		words := strings.TrimSpace(memory.OneLine(m.Concern))
		if words == "" || utf8.RuneCountInString(words) > MaxConcern {
			continue
		}
		taken[m.Field] = true
		out = append(out, Concern{Field: m.Field, Concern: words, Suggestion: suggestion(m.Field, m.Suggestion, bound)})
	}
	return out
}

// suggestion is a model's replacement for field, made safe to put in an
// editor: no control character but a line break (none at all for a one-line
// field), cut at the field's bound.
func suggestion(field, s string, bound int) string {
	s = strings.TrimSpace(s)
	if oneLine[field] {
		s = memory.OneLine(s)
	} else {
		s = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\n' {
				return ' '
			}
			return r
		}, s)
	}
	s = strings.TrimSpace(cut(s, bound))
	if field == "target_date" {
		if _, err := time.Parse(dateLayout, s); err != nil {
			return ""
		}
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
