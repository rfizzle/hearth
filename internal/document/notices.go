package document

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/document/picture"
	"github.com/rfizzle/hearth/internal/document/sheet"
)

// Every notice a read adds is built here, from a fixed sentence in brackets;
// only a file name, page numbers and the time limit are filled in. Nothing a
// document says reaches one.

func noticePassword(name string) string {
	return fmt.Sprintf("[%s is password-protected, so Hearth cannot read it. Ask the owner for a copy without the password.]", name)
}

func noticeNoPages(name string) string {
	return fmt.Sprintf("[%s has no pages Hearth can read.]", name)
}

// noticeUnreadOf is the notice for the parts of a document, called noun in the
// singular, that could not be read.
func noticeUnreadOf(parts []int, noun string) string {
	if len(parts) == 1 {
		return fmt.Sprintf("[%s %s could not be read.]", capital(noun), picture.PageList(parts))
	}
	return fmt.Sprintf("[%ss %s could not be read.]", capital(noun), picture.PageList(parts))
}

func capital(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func noticeTimeLimitOf(limit time.Duration, at, total int, noun string) string {
	secs := int((limit + time.Second - 1) / time.Second)
	if at == 0 {
		return fmt.Sprintf("[Reading stopped after %d seconds, before any %s was read.]", secs, noun)
	}
	return fmt.Sprintf("[Reading stopped after %d seconds at %s %d of %d.]", secs, noun, at, total)
}

func noticeTooManyOf(from, to int, noun string) string {
	return fmt.Sprintf(`[%ss %d to %d were not read; call files.read with pages "%s".]`, capital(noun), from, to, PagesHint(from, to))
}

func noticeTextCapOf(at, to int, noun string) string {
	return fmt.Sprintf(`[Reading stopped at %d MB of text, after %s %d. To read on, call files.read with pages "%s".]`, MaxText>>20, noun, at, PagesHint(at+1, to))
}

// PagesHint is the range to pass as pages to read on from page from, at most
// MaxPages of it, ending no later than page to.
func PagesHint(from, to int) string {
	return fmt.Sprintf("%d-%d", from, min(to, from+MaxPages-1))
}

func noticeEmpty(name string) string {
	return fmt.Sprintf("[%s has no text.]", name)
}

// noticeUnreadFile is a file whose reader gave up on it part way.
func noticeUnreadFile(name string) string {
	return fmt.Sprintf("[%s could not be read all the way through.]", name)
}

// noticeTimeLimitWhole says a read of a file without pages ran out of time.
func noticeTimeLimitWhole(limit time.Duration) string {
	return fmt.Sprintf("[Reading stopped after %d seconds.]", int((limit+time.Second-1)/time.Second))
}

// noticeTextStop says a file without pages was cut at the text cap.
func noticeTextStop() string {
	return fmt.Sprintf("[Reading stopped at %d MB of text.]", MaxText>>20)
}

// noticeLeftOut says what a Word file has that is not read.
func noticeLeftOut(images, footnotes bool) string {
	switch {
	case images && footnotes:
		return "[The document has images and footnotes; they were left out.]"
	case images:
		return "[The document has images; they were left out.]"
	}
	return "[The document has footnotes; they were left out.]"
}

// noticeCellCap says a table was cut at the cell cap.
func noticeCellCap(cells int) string {
	n := strconv.Itoa(cells)
	if len(n) > 3 {
		n = n[:len(n)-3] + "," + n[len(n)-3:]
	}
	return fmt.Sprintf("[The table has more than %s cells; only the first rows, up to %s cells, are shown. Ask the owner for a smaller file.]", n, n)
}

// label is a name taken from a document (a sheet's) made fit for a notice or a
// heading: one line, no brackets or quotation marks to close a notice early,
// and at most labelRunes characters.
func label(s string) string {
	s = strings.ToValidUTF8(oneLine(s), "�")
	s = strings.NewReplacer("[", "(", "]", ")", `"`, "'").Replace(s)
	if r := []rune(s); len(r) > labelRunes {
		s = string(r[:labelRunes]) + "…"
	}
	return strings.TrimSpace(s)
}

// labelRunes is the longest a document's own name is allowed to be in a
// notice, and how many names a notice lists.
const (
	labelRunes = 60
	listNames  = 10
)

// commas writes n with a comma between each three digits, as the owner reads
// a number.
func commas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// nameList is names as `"A", "B" and "C"`, at most listNames of them, then
// the number left over.
func nameList(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	shown := names[:min(len(names), listNames)]
	parts := make([]string, len(shown))
	for i, n := range shown {
		parts[i] = `"` + label(n) + `"`
	}
	more := ""
	if len(names) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(names)-len(shown))
	}
	if len(parts) == 1 {
		return parts[0] + more
	}
	if more != "" {
		return strings.Join(parts, ", ") + more
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// noticeSheetCut says a sheet has more cells than are shown. exact is false
// when the count stopped at the deadline.
func noticeSheetCut(name string, cells int, exact bool) string {
	have := "has " + commas(cells) + " cells"
	if !exact {
		have = "has at least " + commas(cells) + " cells"
	}
	return fmt.Sprintf(`[Sheet "%s" %s; the first %s are shown. Ask the owner for a smaller sheet or a CSV.]`, label(name), have, commas(sheet.MaxCells))
}

func noticeSheetsLeft(left []string) string {
	return fmt.Sprintf(`[Sheets %s were not read; only the first %d sheets are read. Call files.read with sheet "%s" to read one.]`, nameList(left), sheet.MaxSheets, label(left[0]))
}

func noticeSheetUnread(name string) string {
	return fmt.Sprintf(`[Sheet "%s" could not be read.]`, label(name))
}

func noticeSheetTextCap(name string) string {
	return fmt.Sprintf(`[Reading stopped at %d MB of text, in sheet "%s".]`, MaxText>>20, label(name))
}

func noticeCellsCut(runes int) string {
	return fmt.Sprintf("[Cells longer than %d characters were cut.]", runes)
}

func noticeNoSheets(name string) string {
	return fmt.Sprintf("[%s has no sheets Hearth can read.]", name)
}
