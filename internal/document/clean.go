package document

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// appVoice is how a line begins when it speaks as the app; the framing tells
// a bot such a line is from the app and not from the person.
const appVoice = "(hearth)"

// clean makes extracted text safe to hand to a model as data: invalid UTF-8
// is replaced, line and paragraph separators become newlines, every other
// control character but tab and newline is dropped, and a line that would
// begin with the app's voice, in any case and after any indent, gains a
// leading space so it no longer does.
func clean(s string) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n", " ", "\n", " ", "\n").Replace(s)
	s = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimLeft(line, " \t")), appVoice) {
			lines[i] = " " + line
		}
	}
	return strings.Join(lines, "\n")
}

// IsPicturePage is the rule for a page whose text is not text: it has fewer
// than 20 characters that are not white space, or more than a tenth of its
// characters are U+FFFD or not printable (a font with no map to Unicode).
// An empty page is a picture. It judges the text as the parser returned it,
// before cleaning.
func IsPicturePage(text string) bool {
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
