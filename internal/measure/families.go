package measure

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/project"
)

// subjectOf says what the second part of a family's id names.
type subjectOf int

const (
	noSubject     subjectOf = iota // inbox.open, team.size
	aProject                       // a project's id
	aBot                           // a bot's id
	botOrOrg                       // a bot's id, or org
	botOrEveryone                  // a bot's id, or everyone
)

// family is one row of the registry: the first part of an id, the kind and
// unit of its values, what its subject names, whether it takes a month, the
// label's first words and the rule that reads a value.
type family struct {
	name    string
	kind    Kind
	unit    string
	subject subjectOf
	monthly bool
	what    string
	read    func(r *reader, id ID, v *Value) error
}

// families is the registry, in the order List gives it. It is append-only: a
// saved dashboard holds these ids.
//
// Waiting for the task stories, with their names fixed now so a resolver is
// added and no id changes: project.cost/<project id> (needs the project's
// budget field), project.tasks.<state>/<project id> (waiting, failed, and so
// on), tasks.open/<bot id>, and team.working (runs in progress, once a run
// carries its bot in the bus's list).
var families = []family{
	about(aProject, "project.tasks", Progress, "count", "Tasks done", readTasks),
	about(aProject, "project.light", Light, "", "Project status", readLight),
	about(aProject, "project.files", Number, "count", "Files", readFiles),
	spent("usage.cost", Number, "usd_micros", "Cost", readCost),
	spent("usage.tokens", Number, "tokens", "Tokens", readTokens),
	spent("usage.budget", Progress, "cents", "Budget spent", readBudget),
	about(noSubject, "inbox.open", Number, "count", "Open in the Inbox", readInbox),
	about(aBot, "notes.count", Number, "count", "Notes", readNotes),
	about(botOrEveryone, "handbook.meter", Progress, "bytes", "Handbook", readHandbook),
	about(noSubject, "team.size", Number, "count", "Team size", readTeam),
}

// spent is a family of usage: a bot's or the org's, in a month.
func spent(name string, kind Kind, unit, what string, read func(*reader, ID, *Value) error) family {
	f := about(botOrOrg, name, kind, unit, what, read)
	f.monthly = true
	return f
}

// about is a family that takes no month.
func about(subject subjectOf, name string, kind Kind, unit, what string, read func(*reader, ID, *Value) error) family {
	return family{name: name, kind: kind, unit: unit, subject: subject, what: what, read: read}
}

// family is the row id names, or a refusal that lists the families there are.
func (id ID) family() (family, error) {
	for _, f := range families {
		if f.name == id.Family {
			return f, nil
		}
	}
	names := make([]string, len(families))
	for i, f := range families {
		names[i] = f.name
	}
	return family{}, refuse("%q is not a measure; the families are %s", id.Family, strings.Join(names, ", "))
}

// shape is how a family's ids are written.
func (f family) shape() string {
	s := f.name
	switch f.subject {
	case aProject:
		s += "/<project id>"
	case aBot:
		s += "/<bot id>"
	case botOrOrg:
		s += "/<bot id or org>"
	case botOrEveryone:
		s += "/<bot id or everyone>"
	}
	if f.monthly {
		s += "/<yyyy-mm or current>"
	}
	return s
}

// check refuses an id whose subject or month is not what the family takes.
func (f family) check(id ID) error {
	if (f.subject == noSubject) != (id.Subject == "") {
		return refuse("%s is written %s", f.name, f.shape())
	}
	if id.Subject != "" && !plain(id.Subject, 64) {
		return refuse("the subject %q of %s is letters, digits, - and _ (at most 64); write %s", id.Subject, f.name, f.shape())
	}
	if !f.monthly {
		if id.Period != "" {
			return refuse("%s takes no month; write %s", f.name, f.shape())
		}
		return nil
	}
	if id.Period == "" {
		return refuse("%s needs a month; write %s", f.name, f.shape())
	}
	if _, err := time.Parse("2006-01", id.Period); err != nil && id.Period != "current" {
		return refuse("the month %q of %s is yyyy-mm, like 2026-10, or current", id.Period, f.name)
	}
	return nil
}

func plain(s string, limit int) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func readTasks(r *reader, id ID, v *Value) error {
	st, err := r.d.Status(r.ctx, id.Subject)
	if err != nil {
		return err
	}
	v.Done, v.Total = int64(st.Counts.Done), int64(st.Counts.Total)
	if st.Counts.Total == 0 {
		v.Word = st.Label
	}
	return nil
}

func readLight(r *reader, id ID, v *Value) error {
	st, err := r.d.Status(r.ctx, id.Subject)
	if err != nil {
		return err
	}
	v.Word = st.Label
	switch st.Light {
	case project.LightGreen:
		v.Tone = TonePositive
	case project.LightYellow:
		v.Tone = ToneAttention
	case project.LightRed:
		v.Tone = ToneNegative
	default:
		v.Tone = ToneNeutral
	}
	return nil
}

// readFiles counts the regular files directly in the project's folder; a
// folder that is not there yet holds none.
func readFiles(r *reader, id ID, v *Value) error {
	p, _ := r.project(id.Subject)
	if p.FolderPath == "" {
		return nil
	}
	entries, err := os.ReadDir(p.FolderPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			v.Number++
		}
	}
	return nil
}

func readCost(r *reader, id ID, v *Value) error {
	s, err := r.spend(id)
	if err != nil {
		return err
	}
	if !s.Priced {
		v.Missing = "Not priced"
		return nil
	}
	v.Number = float64(s.CostMicros)
	return nil
}

func readTokens(r *reader, id ID, v *Value) error {
	s, err := r.spend(id)
	v.Number = float64(s.Tokens)
	return err
}

// readBudget is the cents spent of the cents allowed; a micro-dollar is a
// millionth, a cent ten thousand of them.
func readBudget(r *reader, id ID, v *Value) error {
	s, err := r.spend(id)
	if err != nil {
		return err
	}
	if s.BudgetCents == 0 {
		v.Missing = "No budget set"
		return nil
	}
	v.Done, v.Total = s.CostMicros/10_000, s.BudgetCents
	return nil
}

func readInbox(r *reader, _ ID, v *Value) error {
	n, err := r.d.Inbox.Open(r.ctx)
	v.Number = float64(n)
	return err
}

func readNotes(r *reader, id ID, v *Value) error {
	n, err := r.d.Notes.Count(r.ctx, id.Subject)
	v.Number = float64(n)
	return err
}

func readHandbook(r *reader, id ID, v *Value) error {
	botID := id.Subject
	if botID == "everyone" {
		botID = ""
	}
	used, limit, err := r.d.Handbook.Used(r.ctx, botID)
	v.Done, v.Total = used, limit
	return err
}

func readTeam(r *reader, _ ID, v *Value) error {
	bots, err := r.bots()
	v.Number = float64(len(bots))
	return err
}
