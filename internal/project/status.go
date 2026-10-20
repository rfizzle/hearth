package project

// Counts is how many of a project's tasks are in each state.
type Counts struct {
	Total, Done, Working, WaitingOwner, PausedBudget, Failed, Cancelled int
}

// Light is the colour a project shows. The values are stored on the wire.
type Light int

// The lights, from nothing to look at to something to act on.
const (
	LightNone   Light = 1
	LightGreen  Light = 2
	LightYellow Light = 3
	LightRed    Light = 4
)

// Status is what the owner reads of a project: the counts, the light, the
// words beside it, and whether every task is finished. The pie's fill is
// Done over Total; the app draws it and the rule names it.
type Status struct {
	Counts  Counts
	Light   Light
	Label   string
	AllDone bool
}

// StatusOf is the one rule, first match wins: no tasks is none; a task
// paused on budget is red; a task waiting on the owner or failed is yellow;
// every task done or cancelled is green and all done; otherwise green. A
// cancelled task counts as finished but not as done.
// See docs/model.md#work.
func StatusOf(c Counts) Status {
	s := Status{Counts: c}
	switch {
	case c.Total == 0:
		s.Light, s.Label = LightNone, "No tasks yet"
	case c.PausedBudget > 0:
		s.Light, s.Label = LightRed, "Paused on budget"
	case c.WaitingOwner+c.Failed > 0:
		s.Light, s.Label = LightYellow, "Waiting on you"
	case c.Done+c.Cancelled == c.Total:
		s.Light, s.Label, s.AllDone = LightGreen, "All tasks done", true
	default:
		s.Light, s.Label = LightGreen, "On track"
	}
	return s
}
