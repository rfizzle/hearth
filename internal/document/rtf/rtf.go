package rtf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"unicode/utf16"
)

// MaxDepth is how deeply groups may nest.
const MaxDepth = 100

// maxDigits is the most digits a control word's number may have.
const maxDigits = 10

// checkEvery is how many bytes are read between looks at the context.
const checkEvery = 64 << 10

var (
	// ErrNotRTF is a file that does not begin as RTF does.
	ErrNotRTF = errors.New("the file is not RTF")
	// ErrTooDeep is a file whose groups nest past MaxDepth.
	ErrTooDeep = errors.New("the RTF nests groups too deeply")
)

// dropped are the destinations whose text is not reading text.
var dropped = wordSet(`fonttbl colortbl stylesheet info pict object fldinst
	listtable listoverridetable revtbl rsidtbl generator filetbl latentstyles
	themedata datastore colorschememapping pgptbl xmlnstbl template bkmkstart
	bkmkend header footer headerl headerr headerf footerl footerr footerf
	footnote private`)

func wordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(words) {
		set[w] = true
	}
	return set
}

// symbols are the control words that stand for one character.
var symbols = map[string]string{
	"par": "\n", "line": "\n", "sect": "\n\n", "page": "\n", "row": "\n",
	"tab": "\t", "cell": "\t", "nestcell": "\t",
	"emdash": "—", "endash": "–", "bullet": "•",
	"lquote": "‘", "rquote": "’", "ldblquote": "“", "rdblquote": "”",
	"emspace": " ", "enspace": " ", "qmspace": " ",
}

// cp1252 is bytes 0x80 to 0x9f of Windows-1252; the rest of the table is
// Latin-1, which is the same as Unicode. A zero is a byte the code page
// leaves unused.
var cp1252 = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

func decodeByte(b byte) rune {
	if b >= 0x80 && b < 0xa0 {
		if r := cp1252[b-0x80]; r != 0 {
			return r
		}
		return '�'
	}
	return rune(b)
}

// frame is one open group: whether it is dropped, and how many characters
// follow a \uN as its plain fallback (\ucN, 1 unless the file says).
type frame struct {
	drop bool
	uc   int
}

// stripper is the state of one walk.
type stripper struct {
	br    *bufio.Reader
	out   strings.Builder
	limit int
	cut   bool
	stack []frame
	skip  int  // fallback characters still to drop after a \uN
	high  rune // a high surrogate waiting for its low half
}

func (s *stripper) top() *frame { return &s.stack[len(s.stack)-1] }

// emit adds text unless the group is dropped; false means the limit is hit.
func (s *stripper) emit(t string) bool {
	if s.top().drop {
		return true
	}
	if s.out.Len()+len(t) > s.limit {
		s.cut = true
		return false
	}
	s.out.WriteString(t)
	return true
}

// emitRune adds one character, joining a UTF-16 pair written as two \uN.
func (s *stripper) emitRune(c rune) bool {
	if s.high != 0 {
		if c >= 0xdc00 && c <= 0xdfff {
			c = utf16.DecodeRune(s.high, c)
		}
		s.high = 0
	}
	if c >= 0xd800 && c <= 0xdbff {
		s.high = c
		return true
	}
	if c >= 0xdc00 && c <= 0xdfff {
		c = '�'
	}
	return s.emit(string(c))
}

// plain adds a character the file spells out, unless it is the fallback of
// the \uN before it.
func (s *stripper) plain(c rune) bool {
	if s.skip > 0 {
		s.skip--
		return true
	}
	return s.emitRune(c)
}

// open starts a group.
func (s *stripper) open() error {
	if len(s.stack) >= MaxDepth {
		return ErrTooDeep
	}
	s.skip = 0
	s.stack = append(s.stack, frame{drop: s.top().drop, uc: s.top().uc})
	return nil
}

func (s *stripper) close() {
	s.skip = 0
	if len(s.stack) > 1 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

// escape handles what follows a backslash; false means the limit is hit.
func (s *stripper) escape() bool {
	c, err := s.br.ReadByte()
	if err != nil {
		return true
	}
	switch {
	case c == '\'':
		hi, e1 := s.br.ReadByte()
		lo, e2 := s.br.ReadByte()
		if v, ok := hexByte(hi, lo); e1 == nil && e2 == nil && ok {
			return s.plain(decodeByte(v))
		}
	case c == '\\' || c == '{' || c == '}':
		return s.plain(rune(c))
	case c == '*':
		s.top().drop = true
	case c == '~':
		return s.emit(" ")
	case c == '_':
		return s.emit("-")
	case c == '\r' || c == '\n':
		return s.emit("\n")
	case isLetter(c):
		return s.word(c)
	}
	return true
}

// word handles a control word whose first letter is first.
func (s *stripper) word(first byte) bool {
	word, param, has, err := controlWord(s.br, first)
	if err != nil {
		return true
	}
	switch {
	case dropped[word]:
		s.top().drop = true
	case word == "u" && has:
		if param < 0 {
			param += 65536
		}
		if param > 0xffff { // not a UTF-16 unit
			param = 0xfffd
		}
		ok := s.emitRune(rune(param))
		s.skip = s.top().uc
		return ok
	case word == "uc" && has:
		s.top().uc = max(param, 0)
	case word == "bin" && has && param > 0:
		_, _ = io.CopyN(io.Discard, s.br, int64(param))
	case symbols[word] != "":
		return s.emit(symbols[word])
	}
	return true
}

// Strip returns the text of the RTF in r, at most limit bytes of it, and
// whether the text was cut at the limit. It reads r once, to its end or until
// the limit; a file that is not RTF is ErrNotRTF and one nested past
// MaxDepth is ErrTooDeep. ctx ending is ctx's error.
func Strip(ctx context.Context, r io.Reader, limit int) (text string, cut bool, err error) {
	br := bufio.NewReaderSize(r, 32<<10)
	if head, _ := br.Peek(5); !bytes.Equal(head, []byte(`{\rtf`)) {
		return "", false, ErrNotRTF
	}
	s := &stripper{br: br, limit: limit, stack: []frame{{uc: 1}}}
	for read := 0; ; read++ {
		if read%checkEvery == checkEvery-1 {
			if err := ctx.Err(); err != nil {
				return "", false, err
			}
		}
		b, rerr := br.ReadByte()
		if rerr != nil {
			break
		}
		ok := true
		switch b {
		case '{':
			if err := s.open(); err != nil {
				return "", false, err
			}
		case '}':
			s.close()
		case '\r', '\n': // line breaks in the file are not text
		case '\\':
			ok = s.escape()
		default:
			ok = s.plain(decodeByte(b))
		}
		if !ok {
			return finish(&s.out), true, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	return finish(&s.out), false, nil
}

// finish is the text with runs of blank lines folded to one blank line.
func finish(b *strings.Builder) string {
	s := b.String()
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// controlWord reads the rest of a control word whose first letter is first:
// its letters, an optional signed number of at most maxDigits digits, and the
// one space that ends it.
func controlWord(br *bufio.Reader, first byte) (word string, param int, has bool, err error) {
	name := []byte{first}
	for {
		c, err := br.ReadByte()
		if err != nil {
			return string(name), 0, false, err
		}
		if !isLetter(c) {
			_ = br.UnreadByte()
			break
		}
		if len(name) < 32 { // a longer word is junk; keep reading it away
			name = append(name, c)
		}
	}
	neg := false
	if p, _ := br.Peek(2); len(p) == 2 && p[0] == '-' && p[1] >= '0' && p[1] <= '9' {
		neg = true
		_, _ = br.ReadByte()
	}
	digits := 0
	for {
		c, err := br.ReadByte()
		if err != nil {
			break
		}
		if c < '0' || c > '9' {
			if c != ' ' { // a space after a word belongs to the word
				_ = br.UnreadByte()
			}
			break
		}
		if digits < maxDigits {
			param = param*10 + int(c-'0')
			digits++
		}
		has = true
	}
	if neg {
		param = -param
	}
	return string(name), param, has, nil
}

func hexByte(hi, lo byte) (byte, bool) {
	h, ok1 := hexVal(hi)
	l, ok2 := hexVal(lo)
	return h<<4 | l, ok1 && ok2
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
