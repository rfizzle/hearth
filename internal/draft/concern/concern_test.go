package concern_test

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

// today is the day the date rule compares with: a test fixes it.
var today = time.Date(2026, 10, 20, 15, 30, 0, 0, time.UTC)

const fullJob = "You look into the questions the owner brings you and report what you find. You keep every brief to one page."

func TestProject_EachRule(t *testing.T) {
	good := project.Brief{Goal: "Send the launch email for the new API tier.", DoneLooksLike: "A pricing page, a launch email. A schedule.", DoneWhen: "The launch email is in 400 inboxes."}
	with := func(f func(*project.Brief)) project.Brief {
		b := good
		f(&b)
		return b
	}
	for _, tc := range []struct {
		name   string
		pname  string
		brief  project.Brief
		date   string
		want   []concern.Concern
		reason string
	}{
		{name: "nothing weak", pname: "API tier", brief: good, date: "2026-11-20"},
		{
			name: "empty done_when suggests the first sentence", pname: "API tier",
			brief: with(func(b *project.Brief) { b.DoneWhen = "  " }),
			want: []concern.Concern{{
				Field:      "done_when",
				Concern:    "I can't tell when this is done yet. One sentence that's true exactly then.",
				Suggestion: "A pricing page, a launch email.",
			}},
		},
		{
			name: "empty done_when with nothing to suggest from", pname: "API tier",
			brief: with(func(b *project.Brief) { b.DoneWhen, b.DoneLooksLike = "", "" }),
			want: []concern.Concern{{
				Field: "done_when", Concern: "I can't tell when this is done yet. One sentence that's true exactly then.",
			}},
		},
		{
			name: "a vague word and no digit", pname: "API tier",
			brief: with(func(b *project.Brief) { b.DoneWhen = "The list is Better than before." }),
			want: []concern.Concern{{
				Field: "done_when", Concern: "“better” is hard to check. A number or a date makes it something we can tick.",
			}},
		},
		{name: "a vague word beside a digit", pname: "API tier", brief: with(func(b *project.Brief) { b.DoneWhen = "Some 400 people read it." })},
		{name: "a vague word only inside a longer word", pname: "API tier", brief: with(func(b *project.Brief) { b.DoneWhen = "Something has shipped." })},
		{
			name: "a date before today", pname: "API tier", brief: good, date: "2026-10-19",
			want: []concern.Concern{{Field: "target_date", Concern: "That date has already passed."}},
		},
		{name: "today is not passed", pname: "API tier", brief: good, date: "2026-10-20"},
		{name: "a date that does not parse is the core's refusal", pname: "API tier", brief: good, date: "last week"},
		{
			name: "a name that restates the goal", pname: "Send the launch email for the API tier",
			brief: good,
			want:  []concern.Concern{{Field: "name", Concern: "The name reads like the goal. A short name is easier to find in the sidebar."}},
		},
		{name: "four words is a short enough name", pname: "Send the launch email", brief: good},
		{name: "a long name with a word the goal lacks", pname: "Send the launch email for the weekly API tier", brief: good},
		{
			name: "three at once, in field order", pname: "Send the launch email for the API tier",
			brief: with(func(b *project.Brief) { b.DoneWhen = "" }), date: "2020-01-01",
			want: []concern.Concern{
				{Field: "name", Concern: "The name reads like the goal. A short name is easier to find in the sidebar."},
				{Field: "done_when", Concern: "I can't tell when this is done yet. One sentence that's true exactly then.", Suggestion: "A pricing page, a launch email."},
				{Field: "target_date", Concern: "That date has already passed."},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := concern.Project(tc.pname, tc.brief, tc.date, today)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Project = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestProject_SuggestionIsOneBoundedSentence(t *testing.T) {
	long := strings.Repeat("word ", 100) + "end."
	got := concern.Project("n", project.Brief{DoneLooksLike: "A\nline\nbreak. Second."}, "", today)
	if len(got) != 1 || got[0].Suggestion != "A line break." {
		t.Errorf("Project = %+v, want the first sentence on one line", got)
	}
	got = concern.Project("n", project.Brief{DoneLooksLike: long}, "", today)
	if len(got) != 1 || utf8.RuneCountInString(got[0].Suggestion) > 201 || !strings.HasSuffix(got[0].Suggestion, "…") {
		t.Errorf("Project = %+v, want a suggestion cut at a word with an ellipsis", got)
	}
}

func TestBot_EachRule(t *testing.T) {
	team := []org.Bot{{Name: "Dana", Role: "Research Lead"}, {Name: "Sam", Role: "Writer"}}
	for _, tc := range []struct {
		name string
		role string
		job  string
		want []concern.Concern
	}{
		{name: "nothing weak", role: "Scheduler", job: fullJob},
		{
			name: "a role another bot has, in another case and padded", role: "  research lead ", job: fullJob,
			want: []concern.Concern{{Field: "role", Concern: "You already have Dana, your Research Lead. A second one is fine if they'll do different work."}},
		},
		{name: "an empty role is not a duplicate", role: " ", job: fullJob},
		{
			name: "under eight words", role: "Scheduler", job: "Keeps the calendar tidy for me.",
			want: []concern.Concern{{Field: "job_description", Concern: thinWords}},
		},
		{
			name: "eleven words and no sentence end", role: "Scheduler", job: "keeps the calendar tidy and books every meeting I ask for",
			want: []concern.Concern{{Field: "job_description", Concern: thinWords}},
		},
		{name: "eleven words with a sentence end", role: "Scheduler", job: "Keeps the calendar tidy and books every meeting I ask for."},
		{name: "twelve words and no sentence end", role: "Scheduler", job: "keeps the calendar tidy and books every meeting that I ask for"},
		{
			name: "an empty job description", role: "Scheduler", job: "",
			want: []concern.Concern{{Field: "job_description", Concern: thinWords}},
		},
		{
			name: "both, in field order", role: "Writer", job: "Writes.",
			want: []concern.Concern{
				{Field: "role", Concern: "You already have Sam, your Writer. A second one is fine if they'll do different work."},
				{Field: "job_description", Concern: thinWords},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := concern.Bot("Wren", tc.role, tc.job, team)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Bot = %+v, want %+v", got, tc.want)
			}
		})
	}
}

const thinWords = "This says what the job is called more than what they'd do. Two or three sentences about the work help them do it well."

func TestBot_QuotesOfStoredTextStayOnOneLineAndWithinTheBound(t *testing.T) {
	team := []org.Bot{{Name: "Da\nna", Role: strings.Repeat("R", 300)}}
	got := concern.Bot("", strings.Repeat("R", 300), fullJob, team)
	if len(got) != 1 || strings.Contains(got[0].Concern, "\n") || utf8.RuneCountInString(got[0].Concern) > concern.MaxConcern {
		t.Errorf("Bot = %q, want one line within %d characters", got, concern.MaxConcern)
	}
}

func c(field string) concern.Concern {
	return concern.Concern{Field: field, Concern: "Words for " + field + "."}
}

func fields(cs []concern.Concern) []string {
	out := []string{}
	for _, x := range cs {
		out = append(out, x.Field)
	}
	return out
}

func TestCap(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rules, model []concern.Concern
		want         []string
	}{
		{name: "rules come first, the model fills the rest", rules: []concern.Concern{c("name")}, model: []concern.Concern{c("goal"), c("context")}, want: []string{"name", "goal", "context"}},
		{name: "at most three, rules before the model", rules: []concern.Concern{c("name"), c("done_when")}, model: []concern.Concern{c("goal"), c("context")}, want: []string{"name", "done_when", "goal"}},
		{name: "a model concern on a field a rule has is dropped", rules: []concern.Concern{c("name")}, model: []concern.Concern{c("name"), c("goal")}, want: []string{"name", "goal"}},
		{name: "a second model concern on one field is dropped", model: []concern.Concern{c("goal"), c("goal"), c("context")}, want: []string{"goal", "context"}},
		{name: "an unknown field is dropped", model: []concern.Concern{c("tier"), c("anything"), c("goal")}, want: []string{"goal"}},
		{name: "empty words are dropped", model: []concern.Concern{{Field: "goal", Concern: " \n "}, c("context")}, want: []string{"context"}},
		{name: "words over the bound are dropped", model: []concern.Concern{{Field: "goal", Concern: strings.Repeat("x", concern.MaxConcern+1)}, c("context")}, want: []string{"context"}},
		{name: "words at the bound stay", model: []concern.Concern{{Field: "goal", Concern: strings.Repeat("x", concern.MaxConcern)}}, want: []string{"goal"}},
		{name: "words are counted after being made one line", model: []concern.Concern{{Field: "goal", Concern: strings.Repeat("x", 199) + "\n\n"}}, want: []string{"goal"}},
		{name: "more rules than slots", rules: []concern.Concern{c("name"), c("role"), c("done_when"), c("target_date")}, want: []string{"name", "role", "done_when"}},
		{name: "nothing in, nothing out", want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fields(concern.Cap(concern.ForProject, tc.rules, tc.model)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Cap fields = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCap_WordsAreOneLineAndSuggestionsAreCutAndClean(t *testing.T) {
	long := strings.Repeat("é", 200)
	got := concern.Cap(concern.ForProject, nil, []concern.Concern{
		{Field: "name", Concern: "Too\nlong.", Suggestion: "  " + strings.Repeat("n", 100) + "\n  "},
		{Field: "goal", Concern: "Be clearer.", Suggestion: "Line one.\nLine\x00 two."},
		{Field: "done_when", Concern: "Vague.", Suggestion: "One\nline " + long},
	})
	if len(got) != 3 {
		t.Fatalf("Cap = %+v", got)
	}
	if got[0].Concern != "Too long." || got[0].Suggestion != strings.Repeat("n", 80) {
		t.Errorf("name = %+v, want one line and a suggestion cut to 80", got[0])
	}
	if got[1].Suggestion != "Line one.\nLine two." {
		t.Errorf("goal suggestion = %q, want a line break kept and the control character dropped", got[1].Suggestion)
	}
	if strings.Contains(got[2].Suggestion, "\n") || got[2].Suggestion != "One line "+long {
		t.Errorf("done_when suggestion = %q, want one line, under its 4,000 bound", got[2].Suggestion)
	}
	huge := concern.Cap(concern.ForProject, nil, []concern.Concern{{Field: "context", Concern: "x", Suggestion: strings.Repeat("é", 5000)}})
	if n := utf8.RuneCountInString(huge[0].Suggestion); n != 4000 {
		t.Errorf("a context suggestion of 5,000 runes is %d, want it cut to 4,000", n)
	}
}

func TestCap_InvisibleCharactersAreDropped(t *testing.T) {
	const invisible = "\u202e\u2066\u2069\u200b\ufeff\U000E0041\x00\u0085"
	got := concern.Cap(concern.ForProject, nil, []concern.Concern{
		{Field: "name", Concern: "Too" + invisible + " long.", Suggestion: "Lau" + invisible + "nch"},
		{Field: "goal", Concern: "Clearer.", Suggestion: "One.\n\tTwo " + invisible + "👩\u200d💻."},
	})
	if len(got) != 2 {
		t.Fatalf("Cap = %+v", got)
	}
	if got[0].Concern != "Too long." || got[0].Suggestion != "Launch" {
		t.Errorf("name = %+v, want the invisible characters dropped", got[0])
	}
	if want := "One.\n Two 👩\u200d💻."; got[1].Suggestion != want {
		t.Errorf("goal suggestion = %q, want %q (a ZWJ kept, a line break kept, the tab a space)", got[1].Suggestion, want)
	}
	// Words made only of invisible characters are empty, so the concern is dropped.
	if got := concern.Cap(concern.ForProject, nil, []concern.Concern{{Field: "goal", Concern: "\u200b\ufeff"}}); len(got) != 0 {
		t.Errorf("Cap = %+v, want a concern of invisible words dropped", got)
	}
}

func TestCap_ATargetDateSuggestionMustBeADay(t *testing.T) {
	got := concern.Cap(concern.ForProject, nil, []concern.Concern{
		{Field: "target_date", Concern: "Soon.", Suggestion: "next Friday"},
		{Field: "goal", Concern: "x"},
	})
	if got[0].Suggestion != "" {
		t.Errorf("a suggestion that is not a day = %q, want none", got[0].Suggestion)
	}
	got = concern.Cap(concern.ForProject, nil, []concern.Concern{{Field: "target_date", Concern: "Soon.", Suggestion: "2026-12-01"}})
	if got[0].Suggestion != "2026-12-01" {
		t.Errorf("a day = %q, want it kept", got[0].Suggestion)
	}
}
