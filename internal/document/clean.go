package document

import (
	"strings"
	"unicode"

	"github.com/rfizzle/hearth/internal/document/picture"
	"github.com/rfizzle/hearth/internal/org/memory"
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
	s = memory.Plain(s)
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

// IsPicturePage is the rule for a page whose text is not text; see
// picture.IsPage.
func IsPicturePage(text string) bool { return picture.IsPage(text) }
