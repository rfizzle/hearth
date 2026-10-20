// Package intro is a bot's introduction: the card the app opens a
// conversation with, saying what the bot is for, how to work with it and
// three things to try.
//
// Responsibility: the introduction's shape, the rule that assembles one for a
// bot hired from a description, and the check a shipped one passes.
//
// Invariants the types do not hold:
//   - An introduction is app-rendered text. It is never a message, never part
//     of a run's context and never sent to a provider; nothing that builds a
//     run reads it.
//   - A shipped introduction has exactly three how lines and three prompts;
//     Validate holds it to that. An assembled one always does.
//
// It imports the standard library only, so org can hold one on a bot.
// See docs/model.md#people
package intro

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Line is one "how to work with me" line: an SF Symbol name and the words.
type Line struct {
	Symbol string `json:"symbol"`
	Text   string `json:"text"`
}

// Introduction is what a bot says about itself. The zero value is none.
type Introduction struct {
	Purpose string   `json:"purpose"`
	How     []Line   `json:"how"`
	Try     []string `json:"try"`
}

// MaxPrompt is the longest example prompt, in characters, so no chip wraps
// on its own.
const MaxPrompt = 48

// Lines is how many how lines and how many prompts an introduction has.
const Lines = 3

// maxSentence is the longest quoted job sentence, in characters.
const maxSentence = 200

// Assemble is the introduction of a bot hired from a description, by rule:
// no model writes it. The purpose names the role and quotes the first
// sentence of the job description in the owner's words, never re-voiced; the
// how lines and prompts are the generic three.
func Assemble(role, jobDescription string) Introduction {
	role = strings.TrimSpace(role)
	purpose := "I'm on your team."
	prompt := "What would you do first on this job?"
	if role != "" {
		purpose = "I'm your " + role + "."
		prompt = "What would you do first as my " + role + "?"
	}
	if s := firstSentence(jobDescription); s != "" {
		purpose += " The job, in your words: “" + s + "”"
	}
	return Introduction{
		Purpose: purpose,
		How: []Line{
			{"text.bubble", "Say what you want in plain words; I ask if something is unclear."},
			{"folder", "Anything I make lands in my folder in your Hearth folder, and I say where."},
			{"note.text", "Tell me things worth remembering and I keep a note; you can see and edit my notes."},
		},
		Try: []string{
			"What do you need from me to get started?",
			prompt,
			"Here's what I'm working on this week: …",
		},
	}
}

// firstSentence runs to the first ., ! or ? followed by a space or the end,
// trimmed, and is cut at the last word boundary before maxSentence
// characters, with an ellipsis, when longer.
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	for i, r := range text {
		if (r == '.' || r == '!' || r == '?') && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\n') {
			text = text[:i+1]
			break
		}
	}
	if utf8.RuneCountInString(text) <= maxSentence {
		return text
	}
	cut := string([]rune(text)[:maxSentence])
	if i := strings.LastIndexAny(cut, " \n"); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:") + "…"
}

// Validate is nil when i is a shipped-quality introduction: a purpose, three
// how lines each with a symbol name and words, and three prompts of at most
// MaxPrompt characters, none empty.
func Validate(i Introduction) error {
	if strings.TrimSpace(i.Purpose) == "" {
		return errors.New("introduction: no purpose")
	}
	if len(i.How) != Lines {
		return fmt.Errorf("introduction: %d how lines, want %d", len(i.How), Lines)
	}
	for n, l := range i.How {
		if !symbolName(l.Symbol) {
			return fmt.Errorf("introduction: how line %d: %q is not a symbol name", n+1, l.Symbol)
		}
		if strings.TrimSpace(l.Text) == "" {
			return fmt.Errorf("introduction: how line %d is empty", n+1)
		}
	}
	if len(i.Try) != Lines {
		return fmt.Errorf("introduction: %d prompts, want %d", len(i.Try), Lines)
	}
	for n, p := range i.Try {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("introduction: prompt %d is empty", n+1)
		}
		if c := utf8.RuneCountInString(p); c > MaxPrompt {
			return fmt.Errorf("introduction: prompt %d is %d characters, at most %d", n+1, c, MaxPrompt)
		}
	}
	return nil
}

// symbolName is whether s looks like an SF Symbol name: lower-case letters,
// digits and dots.
func symbolName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}
