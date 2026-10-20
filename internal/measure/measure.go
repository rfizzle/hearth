package measure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

// ErrInvalid is wrapped by every refusal of an id, in words the person who
// typed it can read.
var ErrInvalid = errors.New("invalid measure")

// Kind is the shape of a value.
type Kind string

// The three kinds.
const (
	Number   Kind = "number"
	Progress Kind = "progress"
	Light    Kind = "light"
)

// Tone is the colour of a light.
type Tone string

// The four tones; a number or a progress has none.
const (
	ToneNeutral   Tone = "neutral"
	TonePositive  Tone = "positive"
	ToneAttention Tone = "attention"
	ToneNegative  Tone = "negative"
)

// Value is one figure. A number holds Number and Unit, a progress Done and
// Total (Unit says of what), a light Tone and Word. Missing says in words why
// there is no value, and then the rest is empty. AsOf is when it was read,
// and zero in a List.
type Value struct {
	ID      string
	Kind    Kind
	Label   string
	Number  float64
	Unit    string
	Done    int64
	Total   int64
	Tone    Tone
	Word    string
	AsOf    time.Time
	Missing string
}

// Scope is what me and this mean for one caller: the calling bot and the
// project of the conversation, either of which may be empty.
type Scope struct{ BotID, ProjectID string }

// Spend is what one subject spent in a month: the estimated cost in
// micro-dollars, the tokens, the budget in cents (0 for none), and whether
// every request was priced.
type Spend struct {
	CostMicros, Tokens, BudgetCents int64
	Priced                          bool
}

// Month is the usage of one month: the org's and each bot's that has any.
type Month struct {
	Org  Spend
	Bots map[string]Spend
}

// The sources a measure reads, each small so a test fakes it. Every one is
// the owner's own data and the first four are what the store keeps.

// Roster is the bots; the store is one.
type Roster interface {
	Bots(ctx context.Context) ([]org.Bot, error)
}

// Projects is the projects, archived ones left out unless asked for; the
// store is one.
type Projects interface {
	Projects(ctx context.Context, includeArchived bool) ([]project.Project, error)
}

// Ledger is a month of usage ("2026-10").
type Ledger interface {
	Month(ctx context.Context, month string) (Month, error)
}

// Inbox counts what waits on the owner.
type Inbox interface {
	Open(ctx context.Context) (int, error)
}

// Notes counts the notes a bot keeps.
type Notes interface {
	Count(ctx context.Context, botID string) (int, error)
}

// Handbook says how full a reader's handbook is, in bytes against the cap; a
// botID of "" is the pages for everyone.
type Handbook interface {
	Used(ctx context.Context, botID string) (used, limit int64, err error)
}

// Deps is what a measure is read from. Every field is set; Now is the clock
// for the month current means and for AsOf, and a test pins it.
type Deps struct {
	Roster   Roster
	Projects Projects
	// Status is a project's status, derived as the owner sees it.
	Status   func(ctx context.Context, projectID string) (project.Status, error)
	Ledger   Ledger
	Inbox    Inbox
	Notes    Notes
	Handbook Handbook
	Now      func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// month is the "yyyy-mm" of the clock, in UTC as the meter's months are.
func (d Deps) month() string { return d.now().UTC().Format("2006-01") }

// Resolve turns the aliases of id into what they mean for s: this is the
// conversation's project, me the calling bot, org the org (the pages for
// everyone in a handbook meter), current this month. An alias that means
// nothing for the caller is refused in words; an id with none comes back as
// it is.
func (d Deps) Resolve(id ID, s Scope) (ID, error) {
	f, err := id.family()
	if err != nil {
		return ID{}, err
	}
	switch id.Subject {
	case "this":
		if s.ProjectID == "" {
			return ID{}, refuse("this means the project of the conversation, and this one is not in a project; name the project by its id")
		}
		id.Subject = s.ProjectID
	case "me":
		if s.BotID == "" {
			return ID{}, refuse("me means the bot that is asking; there is none here")
		}
		id.Subject = s.BotID
	case "org":
		if f.subject == botOrEveryone {
			id.Subject = "everyone"
		}
	}
	if id.Period == "current" {
		id.Period = d.month()
	}
	return id, f.check(id)
}

func refuse(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Read is one value per id, in order. Every id is resolved: one still
// holding an alias is ErrInvalid. A subject that is gone is a Value with
// Missing set; an error is a source that could not be read.
func (d Deps) Read(ctx context.Context, ids []ID) ([]Value, error) {
	r := &reader{ctx: ctx, d: d, at: d.now()}
	out := make([]Value, 0, len(ids))
	for _, id := range ids {
		f, err := id.family()
		if err == nil {
			err = f.check(id)
		}
		if err == nil && (isAlias(id.Subject) || id.Period == "current") {
			err = refuse("%s still holds an alias; resolve it first", id)
		}
		if err != nil {
			return nil, err
		}
		v := Value{ID: id.String(), Kind: f.kind, Unit: f.unit, AsOf: r.at}
		v.Label = r.label(f, id)
		if missing, err := r.subject(f, id); err != nil {
			return nil, err
		} else if missing != "" {
			v.Missing = missing
		} else if err := f.read(r, id, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func isAlias(s string) bool { return s == "this" || s == "me" }

// List is every id that resolves now, in registry order, with its label and
// no value: one per project that is not archived, one per bot, and the org
// and everyone where a family takes them. Usage is for the current month.
func (d Deps) List(ctx context.Context) ([]Value, error) {
	r := &reader{ctx: ctx, d: d}
	var out []Value
	for _, f := range families {
		subjects, err := r.subjects(f)
		if err != nil {
			return nil, err
		}
		for _, sub := range subjects {
			id := ID{Family: f.name, Subject: sub}
			if f.monthly {
				id.Period = d.month()
			}
			out = append(out, Value{ID: id.String(), Kind: f.kind, Unit: f.unit, Label: r.label(f, id)})
		}
	}
	return out, nil
}

// ID is a parsed measure id: a family, the subject it is about, and for a
// monthly family the month.
type ID struct{ Family, Subject, Period string }

// String is the id as written, "family/subject/period" without the parts a
// family does not take.
func (id ID) String() string {
	s := id.Family
	if id.Subject != "" {
		s += "/" + id.Subject
	}
	if id.Period != "" {
		s += "/" + id.Period
	}
	return s
}

// ParseID reads "family/subject[/period]" and refuses what the registry
// cannot read in words: an unknown family, a missing or extra part, a month
// that is not yyyy-mm or current. It accepts the aliases; Resolve turns them
// into ids.
func ParseID(s string) (ID, error) {
	if strings.TrimSpace(s) == "" {
		return ID{}, refuse("an id is family/subject, like project.tasks/<project id>")
	}
	if len(s) > 200 {
		return ID{}, refuse("an id is at most 200 characters")
	}
	parts := strings.Split(s, "/")
	id := ID{Family: parts[0]}
	if len(parts) > 1 {
		id.Subject = parts[1]
	}
	if len(parts) > 2 {
		id.Period = parts[2]
	}
	f, err := id.family()
	if err != nil {
		return ID{}, err
	}
	if len(parts) > 3 {
		return ID{}, refuse("%q has more parts than %s takes (%s)", s, f.name, f.shape())
	}
	return id, f.check(id)
}
