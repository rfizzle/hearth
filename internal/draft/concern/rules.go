package concern

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/project"
)

// Project is what looks weak about a project's fields, by rule, in field
// order: the name, then what done is, then the date. today is the day a target
// date is compared with; its UTC day counts.
func Project(name string, brief project.Brief, targetDate string, today time.Time) []Concern {
	var out []Concern
	if nameRestatesGoal(name, brief.Goal) {
		out = append(out, Concern{Field: "name", Concern: "The name reads like the goal. A short name is easier to find in the sidebar."})
	}
	switch done := strings.TrimSpace(brief.DoneWhen); {
	case done == "":
		out = append(out, Concern{
			Field:      "done_when",
			Concern:    "I can't tell when this is done yet. One sentence that's true exactly then.",
			Suggestion: firstSentence(brief.DoneLooksLike),
		})
	case !hasDigit(done):
		if w := vagueWord(done); w != "" {
			out = append(out, Concern{
				Field:   "done_when",
				Concern: "“" + w + "” is hard to check. A number or a date makes it something we can tick.",
			})
		}
	}
	if d, err := time.Parse(dateLayout, targetDate); err == nil && d.Before(day(today)) {
		out = append(out, Concern{Field: "target_date", Concern: "That date has already passed."})
	}
	return out
}

// Bot is what looks weak about a hire, by rule, in field order: the role
// against the bots the org has, then the job description. A role another bot
// holds is compared trimmed and without regard to case. The name is not
// judged; it is a parameter so the call reads as the request does.
func Bot(_, role, jobDescription string, existing []org.Bot) []Concern {
	var out []Concern
	if r := strings.TrimSpace(role); r != "" {
		for _, b := range existing {
			if strings.EqualFold(strings.TrimSpace(b.Role), r) {
				out = append(out, Concern{
					Field: "role",
					Concern: fit("You already have " + memory.OneLine(b.Name) + ", your " + memory.OneLine(b.Role) +
						". A second one is fine if they'll do different work."),
				})
				break
			}
		}
	}
	if thin(jobDescription) {
		out = append(out, Concern{
			Field:   "job_description",
			Concern: "This says what the job is called more than what they'd do. Two or three sentences about the work help them do it well.",
		})
	}
	return out
}

// vague are the words that make a "done when" something no one can tick.
var vague = map[string]bool{
	"better": true, "improved": true, "more": true, "less": true, "good": true,
	"nice": true, "some": true, "happy": true, "enough": true,
}

// vagueWord is the first of the vague words in s, lower-cased, or empty.
func vagueWord(s string) string {
	for _, w := range words(s) {
		if w = strings.ToLower(w); vague[w] {
			return w
		}
	}
	return ""
}

func hasDigit(s string) bool { return strings.ContainsFunc(s, unicode.IsDigit) }

// words are the runs of letters and digits in s.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// nameRestatesGoal is whether the name has five or more words and every one
// of them is in the goal's first sentence.
func nameRestatesGoal(name, goal string) bool {
	n := words(strings.ToLower(name))
	if len(n) < 5 {
		return false
	}
	in := map[string]bool{}
	for _, w := range words(strings.ToLower(sentence(goal))) {
		in[w] = true
	}
	for _, w := range n {
		if !in[w] {
			return false
		}
	}
	return true
}

// thin is whether a job description is too short to say what the work is:
// under eight words, or under twelve with no sentence end.
func thin(s string) bool {
	n := len(strings.Fields(s))
	return n < 8 || (n < 12 && !strings.ContainsAny(s, ".!?"))
}

// day is the start of t's UTC day.
func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// sentence is text up to and with the first ., ! or ? followed by a space, a
// line break or the end, trimmed.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	for i, r := range text {
		if (r == '.' || r == '!' || r == '?') && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\n') {
			return text[:i+1]
		}
	}
	return text
}

// maxSuggestion is the longest suggestion a rule makes, in characters: one
// sentence, not a paragraph.
const maxSuggestion = 200

// firstSentence is sentence on one line, cut at the last word boundary before
// maxSuggestion characters with an ellipsis when longer. It is a bot's
// introduction's rule, copied because this package may not import intro.
func firstSentence(text string) string {
	s := memory.OneLine(sentence(text))
	if utf8.RuneCountInString(s) <= maxSuggestion {
		return s
	}
	c := cut(s, maxSuggestion)
	if i := strings.LastIndex(c, " "); i > 0 {
		c = c[:i]
	}
	return strings.TrimRight(c, " .,;:") + "…"
}

// fit is s cut to MaxConcern characters: a rule's words are fixed, but the
// name and role they quote are the owner's.
func fit(s string) string {
	if utf8.RuneCountInString(s) <= MaxConcern {
		return s
	}
	return strings.TrimRight(cut(s, MaxConcern-1), " ") + "…"
}
