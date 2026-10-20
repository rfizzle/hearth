// Package memory holds what a bot remembers and what every bot reads: the
// notes a bot keeps, the handbook pages the owner writes, and the two pure
// rules over them, which pages a bot reads and how full the handbook is.
//
// Responsibility: plain types and pure rules. The store keeps the rows and
// the orchestrator builds the prompt from them.
//
// Invariants the types do not hold:
//   - The package imports org (when it needs to) and the standard library
//     only, and does no I/O.
//   - Sizes are bytes, not tokens: the core has no tokenizer.
//
// See docs/model.md#7-memory-writes.
package memory

import (
	"strings"
	"time"
	"unicode"
)

// The limits: how many notes a bot keeps before it is asked to tidy them,
// and how many bytes of handbook go into every run.
const (
	Cap         = 200
	HandbookCap = 24 << 10
)

// Note is one thing a bot remembers, in plain words. ConversationID and
// MessageID name where it was learned, when they are known.
type Note struct {
	ID             string
	BotID          string
	Text           string
	Pinned         bool
	ConversationID string
	MessageID      string
	CreatedAt      time.Time
}

// Scope is who reads a page: everyone, or the one bot in BotID.
type Scope struct {
	Everyone bool
	BotID    string
}

// Page is one handbook page: Markdown the owner writes for a scope.
type Page struct {
	ID        string
	Title     string
	Markdown  string
	Scope     Scope
	UpdatedAt time.Time
}

// Select is the pages the bot with botID reads: those for everyone, then
// those for the bot, each group in the order given. It does not change pages.
func Select(pages []Page, botID string) []Page {
	var all, own []Page
	for _, p := range pages {
		switch {
		case p.Scope.Everyone:
			all = append(all, p)
		case botID != "" && p.Scope.BotID == botID:
			own = append(own, p)
		}
	}
	return append(all, own...)
}

// Meter is how many bytes of Markdown the pages take and the most the
// handbook may take.
func Meter(pages []Page) (used, limit int64) {
	for _, p := range pages {
		used += int64(len(p.Markdown))
	}
	return used, HandbookCap
}

// Plain is s with the characters that change how text reads or is read by a
// model without showing: every format character (Unicode category Cf: the
// bidirectional overrides and isolates, the zero-width spaces and joiners,
// the byte order mark, the tag characters) except the zero-width joiner
// U+200D, which emoji sequences need, and every control character (category
// Cc) except the line break and the tab. They are dropped, not replaced, so
// a word split by one rejoins. Text from a model, or typed by the owner, is
// passed through it before it is stored or put in a later prompt.
func Plain(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t' || r == zeroWidthJoiner:
			return r
		case unicode.In(r, unicode.Cf, unicode.Cc):
			return -1
		}
		return r
	}, s)
}

const zeroWidthJoiner = '\u200d'

// OneLine is s with every control character and line or paragraph
// separator, a line break among them, made a space, so text from a note, a
// page title or a file name cannot start a line of its own where it is put.
func OneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, s)
}
