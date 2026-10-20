package deliverable

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// The bounds on Markdown the parser is given. Each closes an input whose
// parse costs the square of its size, in time or in memory, and that no
// deadline can interrupt once the parser has it: nesting, link brackets,
// emphasis delimiters and table cells.
const (
	// maxLeading is the longest run of indentation and list or quote
	// markers a line may start with. Every list or quote a line is inside
	// takes at least one of these bytes, so the run bounds the nesting.
	maxLeading = 256
	// maxBrackets is the most [ a source may hold: 100,000 open links take
	// the parser about a minute.
	maxBrackets = 10_000
	// maxDelimiters is the most *, _ and ~ a source may hold: mismatched
	// emphasis costs the square of their number, and this many parse in
	// about two seconds.
	maxDelimiters = 40_000
	// maxColumns is the most columns a table's delimiter row may have.
	maxColumns = 64
	// maxCells bounds the cells of every table together, counted as the
	// columns of each delimiter row times the lines that follow it up to a
	// blank line: the parser pads every short row to the full width.
	maxCells = 100_000
)

// ErrTooComplex is Markdown the parser is not given: it would cost the
// parser minutes or gigabytes. Each refusal wraps it and says which bound.
var ErrTooComplex = errors.New("the Markdown is too complex to read")

// CheckMarkdown refuses src before it is parsed when it is past one of the
// bounds above. Every caller of ParseMarkdown with text it did not write
// calls this first.
func CheckMarkdown(src []byte) error {
	if n := bytes.Count(src, []byte("[")); n > maxBrackets {
		return tooComplex("it has %d square brackets; the most is %d", n, maxBrackets)
	}
	if n := countAny(src, "*_~"); n > maxDelimiters {
		return tooComplex("it has %d of the characters *, _ and ~; the most is %d", n, maxDelimiters)
	}
	cells, columns := 0, 0
	for line := range bytes.Lines(src) {
		lead := leading(line)
		if lead > maxLeading {
			return tooComplex("a line nests lists or quotes more than %d deep", maxLeading)
		}
		body := bytes.TrimSpace(line[lead:])
		switch {
		case len(body) == 0:
			columns = 0
		case delimiterRow(body):
			columns = bytes.Count(body, []byte("|")) + 1
			if columns > maxColumns+2 { // a leading and a trailing | add one each
				return tooComplex("a table has more than %d columns", maxColumns)
			}
		default:
			if cells += columns; cells > maxCells {
				return tooComplex("its tables have more than %d cells", maxCells)
			}
		}
	}
	return nil
}

func tooComplex(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrTooComplex, fmt.Sprintf(format, args...))
}

func countAny(src []byte, chars string) int {
	n := 0
	for _, c := range src {
		if strings.IndexByte(chars, c) >= 0 {
			n++
		}
	}
	return n
}

// delimiterRow reports whether a line reads as a table's delimiter row:
// only |, -, : and spaces, with a | and a -.
func delimiterRow(body []byte) bool {
	for _, c := range body {
		if c != '|' && c != '-' && c != ':' && c != ' ' && c != '\t' {
			return false
		}
	}
	return bytes.IndexByte(body, '|') >= 0 && bytes.IndexByte(body, '-') >= 0
}

// leading is how many bytes line starts with that could open or hold a
// list or a quote: spaces, tabs, >, and a list marker followed by a space.
func leading(line []byte) int {
	i := 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '>':
			i++
		case c == '-' || c == '*' || c == '+':
			if !spaced(line, i+1) {
				return i
			}
			i += 2
		case c >= '0' && c <= '9':
			j := i
			for j < len(line) && j-i < 10 && line[j] >= '0' && line[j] <= '9' {
				j++
			}
			if j >= len(line) || (line[j] != '.' && line[j] != ')') || !spaced(line, j+1) {
				return i
			}
			i = j + 2
		default:
			return i
		}
	}
	return i
}

func spaced(line []byte, i int) bool {
	return i < len(line) && (line[i] == ' ' || line[i] == '\t')
}
