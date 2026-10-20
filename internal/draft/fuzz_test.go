package draft

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
)

// The seeds are the replies the parser is built for and the ones it must
// refuse: the strict reply, a fenced one, one after a sentence, an array, an
// object with a field it does not know, and a name of 10,000 characters.
var botSeeds = []string{
	`{"name": "Wren", "role": "Research Analyst", "job_description": "You find sources.", "working_style": "Direct", "tier": "best"}`,
	"```json\n" + `{"name": "Wren", "job_description": "You find sources."}` + "\n```",
	`Here you go: {"name": "Wren", "job_description": "You find sources."} Hope that helps.`,
	`["name", "job_description"]`,
	`{"job_description": "You help.", "extra": 1}`,
	`{"name": "` + strings.Repeat("n", 10000) + `", "job_description": "You help."}`,
	`{"job_description": "You help."} {"job_description": "again"}`,
}

var projectSeeds = []string{
	`{"name": "Launch", "goal": "Ship it.", "context": "Small team.", "done_looks_like": "A page.", "done_when": "The page loads.", "target_date": "2026-12-01"}`,
	"```json\n" + `{"goal": "Ship it.", "done_when": "The page loads."}` + "\n```",
	`Here is the project: {"goal": "Ship it.", "done_when": "The page loads.", "target_date": ""} Anything else?`,
	`["goal", "done_when"]`,
	`{"goal": "Ship it.", "done_when": "The page loads.", "concerns": []}`,
	`{"name": "` + strings.Repeat("n", 10000) + `", "goal": "Ship it.", "done_when": "Done.\nreally"}`,
	`{"goal": "Ship it.", "done_when": "x", "target_date": "tomorrow"}`,
}

func FuzzParse(f *testing.F) {
	for _, s := range botSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		r, err := parse(raw, org.TierBalanced)
		if err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("parse(%q) error = %v, want ErrMalformed", raw, err)
			}
			return
		}
		if r.JobDescription == "" {
			t.Fatalf("parse(%q) served no job description", raw)
		}
		within(t, "name", r.Name, maxName, true)
		within(t, "role", r.Role, maxRole, true)
		within(t, "working style", r.WorkingStyle, maxStyle, true)
		within(t, "job description", r.JobDescription, maxPart, false)
		if r.Tier < org.TierFast || r.Tier > org.TierBest {
			t.Fatalf("parse(%q) tier = %v, want one of the three", raw, r.Tier)
		}
	})
}

func FuzzParseProject(f *testing.F) {
	for _, s := range projectSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		r, err := parseProject(raw)
		if err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("parseProject(%q) error = %v, want ErrMalformed", raw, err)
			}
			return
		}
		if r.Brief.Goal == "" || r.Brief.DoneWhen == "" {
			t.Fatalf("parseProject(%q) served a brief without a goal or a done-when line", raw)
		}
		within(t, "name", r.Name, maxName, true)
		within(t, "goal", r.Brief.Goal, maxPart, false)
		within(t, "context", r.Brief.Context, maxPart, false)
		within(t, "done looks like", r.Brief.DoneLooksLike, maxPart, false)
		within(t, "done when", r.Brief.DoneWhen, maxPart, true)
		if r.TargetDate != "" {
			if _, err := time.Parse(dateLayout, r.TargetDate); err != nil {
				t.Fatalf("parseProject(%q) target date = %q, want a day or nothing", raw, r.TargetDate)
			}
		}
	})
}

// within fails the test when s is longer than n characters, is not trimmed or,
// for a one-line field, holds a control character.
func within(t *testing.T, what, s string, n int, oneLine bool) {
	t.Helper()
	if c := utf8.RuneCountInString(s); c > n {
		t.Fatalf("%s has %d characters, want at most %d", what, c, n)
	}
	if s != strings.TrimSpace(s) {
		t.Fatalf("%s = %q, want it trimmed", what, s)
	}
	if oneLine && strings.ContainsFunc(s, func(r rune) bool { return r < ' ' || r == 0x7f || r == 0x2028 || r == 0x2029 }) {
		t.Fatalf("%s = %q, want one line", what, s)
	}
}
