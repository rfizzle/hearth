package measure

import (
	"context"
	"slices"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

// reader is one Read or List: it asks each source once and keeps the answer,
// so a dashboard of forty figures lists the bots once.
type reader struct {
	ctx    context.Context
	d      Deps
	at     time.Time
	roster once[org.Bot]
	projs  once[project.Project]
	months map[string]Month
}

// once is a list asked for the first time and kept after.
type once[T any] struct {
	list   []T
	loaded bool
}

func (o *once[T]) get(load func() ([]T, error)) ([]T, error) {
	if !o.loaded {
		list, err := load()
		if err != nil {
			return nil, err
		}
		o.list, o.loaded = list, true
	}
	return o.list, nil
}

func (r *reader) bots() ([]org.Bot, error) {
	return r.roster.get(func() ([]org.Bot, error) { return r.d.Roster.Bots(r.ctx) })
}

func (r *reader) projects() ([]project.Project, error) {
	return r.projs.get(func() ([]project.Project, error) { return r.d.Projects.Projects(r.ctx, false) })
}

// project is the project with id among those not archived.
func (r *reader) project(id string) (project.Project, bool) {
	return find(r.projects, func(p project.Project) bool { return p.ID == id })
}

func (r *reader) bot(id string) (org.Bot, bool) {
	return find(r.bots, func(b org.Bot) bool { return b.ID == id })
}

// find is the first of the list load returns that is wants, if any.
func find[T any](load func() ([]T, error), wants func(T) bool) (T, bool) {
	list, _ := load()
	if i := slices.IndexFunc(list, wants); i >= 0 {
		return list[i], true
	}
	var none T
	return none, false
}

// subject is the words for a subject that is not there, or "" when it is.
func (r *reader) subject(f family, id ID) (string, error) {
	switch f.subject {
	case aProject:
		if _, err := r.projects(); err != nil {
			return "", err
		}
		if _, ok := r.project(id.Subject); !ok {
			return NoSuchProject, nil
		}
	case aBot, botOrOrg, botOrEveryone:
		if _, err := r.bots(); err != nil {
			return "", err
		}
		if _, ok := r.bot(id.Subject); !ok && !f.special(id.Subject) {
			return NoSuchBot, nil
		}
	}
	return "", nil
}

// special is the subject that is not a bot but a family takes: org or
// everyone.
func (f family) special(s string) bool {
	return f.subject == botOrOrg && s == "org" || f.subject == botOrEveryone && s == "everyone"
}

// subjects is every subject of f that exists now, for List.
func (r *reader) subjects(f family) ([]string, error) {
	var out []string
	switch f.subject {
	case noSubject:
		return []string{""}, nil
	case aProject:
		list, err := r.projects()
		for _, p := range list {
			out = append(out, p.ID)
		}
		return out, err
	case botOrOrg:
		out = append(out, "org")
	case botOrEveryone:
		out = append(out, "everyone")
	}
	list, err := r.bots()
	for _, b := range list {
		out = append(out, b.ID)
	}
	return out, err
}

// label names the figure for a person: what it is, whose, and for usage the
// month. A subject the sources do not know is named by its id.
func (r *reader) label(f family, id ID) string {
	s := f.what
	if id.Subject != "" {
		s += " · " + r.name(f, id.Subject)
	}
	if id.Period != "" {
		s += " · " + id.Period
	}
	return s
}

func (r *reader) name(f family, subject string) string {
	switch {
	case f.special(subject):
		if subject == "org" {
			return "The org"
		}
		return "Everyone"
	case f.subject == aProject:
		if p, ok := r.project(subject); ok {
			return p.Name
		}
	default:
		if b, ok := r.bot(subject); ok {
			return b.Name
		}
	}
	return subject
}

// spend is what the subject of id spent in its month: nothing, priced, for a
// bot that spent nothing.
func (r *reader) spend(id ID) (Spend, error) {
	m, ok := r.months[id.Period]
	if !ok {
		var err error
		if m, err = r.d.Ledger.Month(r.ctx, id.Period); err != nil {
			return Spend{}, err
		}
		if r.months == nil {
			r.months = map[string]Month{}
		}
		r.months[id.Period] = m
	}
	if id.Subject == "org" {
		return m.Org, nil
	}
	if s, ok := m.Bots[id.Subject]; ok {
		return s, nil
	}
	return Spend{Priced: true}, nil
}
