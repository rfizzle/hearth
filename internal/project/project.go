package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
)

// CanPropose is the capability to propose a project to the owner. It is
// defined here rather than in org, whose exports are at their ceiling.
const CanPropose org.Capability = "project.propose"

// State is where a project is in its life.
type State int

// The states a project is in. The values are stored.
const (
	Proposed State = 1
	Active   State = 2
	Done     State = 3
	Archived State = 4
)

// Brief is what the owner and the Chief of Staff agree a project is for.
type Brief struct {
	Goal          string
	Context       string
	DoneLooksLike string
	DoneWhen      string
}

// Project is a goal with a folder and a conversation, and the tasks that
// move it. FolderPath and ConversationID are empty while it is proposed.
type Project struct {
	ID                       string
	Name                     string
	Brief                    Brief
	State                    State
	TargetDate               string // yyyy-mm-dd, or empty
	FolderPath               string
	ConversationID           string
	ProposedByBotID          string
	ProposedInConversationID string
	CreatedAt                time.Time
	ActivatedAt              time.Time
	DoneAt                   time.Time
	ArchivedAt               time.Time
	UpdatedAt                time.Time
}

// ErrInvalid is returned, wrapped with the reason, for a name, brief or date
// a project cannot have.
var ErrInvalid = errors.New("invalid project")

// ErrState is returned, wrapped with the reason, for a move the project's
// state does not allow.
var ErrState = errors.New("project state does not allow that")

const (
	maxName = 80
	maxPart = 4000
	// folder is the folder every project's folder sits in.
	folder     = "Projects"
	dateLayout = "2006-01-02"
)

// Validate is nil when name, brief and targetDate may make a project: a name
// of one to 80 runes on one line, four brief parts of one to 4,000 runes each, and a
// target date that is empty or yyyy-mm-dd.
func Validate(name string, brief Brief, targetDate string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: the name is empty", ErrInvalid)
	}
	if memory.OneLine(name) != name {
		return fmt.Errorf("%w: the name has a line break or a control character", ErrInvalid)
	}
	if utf8.RuneCountInString(name) > maxName {
		return fmt.Errorf("%w: the name is over %d characters", ErrInvalid, maxName)
	}
	parts := []struct{ what, text string }{
		{"the goal", brief.Goal},
		{"the context", brief.Context},
		{"what done looks like", brief.DoneLooksLike},
		{"when it is done", brief.DoneWhen},
	}
	for _, p := range parts {
		if strings.TrimSpace(p.text) == "" {
			return fmt.Errorf("%w: %s is empty", ErrInvalid, p.what)
		}
		if utf8.RuneCountInString(p.text) > maxPart {
			return fmt.Errorf("%w: %s is over %d characters", ErrInvalid, p.what, maxPart)
		}
	}
	if targetDate != "" {
		if _, err := time.Parse(dateLayout, targetDate); err != nil {
			return fmt.Errorf("%w: the target date %q is not yyyy-mm-dd", ErrInvalid, targetDate)
		}
	}
	return nil
}

// New is an active project as the owner creates it: the name trimmed, the
// brief as given, the folder under root/Projects, and its conversation. The
// ids and times are the store's to fill. It refuses what Validate refuses.
func New(name string, brief Brief, targetDate, root string) (Project, org.Conversation, error) {
	if err := Validate(name, brief, targetDate); err != nil {
		return Project{}, org.Conversation{}, err
	}
	p := Project{
		Name:       strings.TrimSpace(name),
		Brief:      brief,
		State:      Active,
		TargetDate: targetDate,
	}
	p.FolderPath = filepath.Join(root, folder, org.FolderName(p.Name))
	return p, org.Conversation{Kind: org.ConversationProject, Title: p.Name}, nil
}

// Propose is a project as the Chief of Staff proposes it: the same
// validation as New, no folder, no conversation, State Proposed.
func Propose(name string, brief Brief, byBotID, inConversationID string) (Project, error) {
	if err := Validate(name, brief, ""); err != nil {
		return Project{}, err
	}
	return Project{
		Name:                     strings.TrimSpace(name),
		Brief:                    brief,
		State:                    Proposed,
		ProposedByBotID:          byBotID,
		ProposedInConversationID: inConversationID,
	}, nil
}

// CanBecome is nil when a project in state s may move to next: done only
// from active, archived from any state but archived (on a proposal it is
// "Not now"). Confirming a proposal goes through Activate.
func (s State) CanBecome(next State) error {
	switch {
	case next == Done && s == Active:
		return nil
	case next == Archived && s != Archived:
		return nil
	}
	return fmt.Errorf("%w: it cannot go from %s to %s", ErrState, s, next)
}

// String is the state's word, for the owner and the log.
func (s State) String() string {
	switch s {
	case Proposed:
		return "proposed"
	case Active:
		return "active"
	case Done:
		return "done"
	case Archived:
		return "archived"
	}
	return fmt.Sprintf("state %d", int(s))
}

// Edit is p with the owner's edits applied: the name trimmed, the brief and
// the target date replaced. It refuses what Validate refuses. The folder is
// p's and stays.
func Edit(p Project, name string, brief Brief, targetDate string) (Project, error) {
	if err := Validate(name, brief, targetDate); err != nil {
		return Project{}, err
	}
	p.Name, p.Brief, p.TargetDate = strings.TrimSpace(name), brief, targetDate
	return p, nil
}

// Activate is p confirmed: State Active with the edits applied, the folder
// and the conversation as New gives them. The id and the proposer are p's.
// Only a proposed project may be confirmed.
func Activate(p Project, name string, brief Brief, targetDate, root string) (Project, org.Conversation, error) {
	if p.State != Proposed {
		return Project{}, org.Conversation{}, fmt.Errorf("%w: only a proposal can be confirmed, this one is %s", ErrState, p.State)
	}
	n, conv, err := New(name, brief, targetDate, root)
	if err != nil {
		return Project{}, org.Conversation{}, err
	}
	p.Name, p.Brief, p.TargetDate, p.FolderPath, p.State = n.Name, n.Brief, n.TargetDate, n.FolderPath, Active
	conv.ProjectID = p.ID
	return p, conv, nil
}
