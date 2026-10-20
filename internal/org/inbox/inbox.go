package inbox

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/org"
)

// Kind is what an item asks of the owner.
type Kind int

// The kinds of item.
const (
	Approval Kind = 1
	Question Kind = 2
	Failure  Kind = 3
	Budget   Kind = 4
)

// State is whether an item still waits.
type State int

// The states of an item.
const (
	Open   State = 1
	Closed State = 2
)

// Decision is the owner's answer to an approval. The zero value is none.
type Decision int

// The four decisions.
// See docs/model.md#8-approvals.
const (
	AllowOnce    Decision = 1
	AllowForTask Decision = 2
	AlwaysAllow  Decision = 3
	Deny         Decision = 4
)

// Item is one thing waiting for the owner, or one that has been dealt with.
// Effect is the plain-words line the owner reads; Call is the tool call an
// approval would run. Widget and WidgetAnswer are the JSON of the control a
// question asks with and the answer to it. Decision, Answer and Outcome are
// set when the item closes.
type Item struct {
	ID             string
	Kind           Kind
	State          State
	BotID          string
	ConversationID string
	RunID          string
	MessageID      string
	Title          string
	Effect         string
	Call           *org.ToolCall
	Question       string
	Suggested      []string
	Widget         string
	WidgetAnswer   string
	CreatedAt      time.Time
	ClosedAt       time.Time
	Decision       Decision
	Answer         string
	Outcome        string
}

// Allowance lets a conversation use one capability in one folder without
// asking again, until the conversation's task ends. Folder is empty for a
// capability with no folder.
type Allowance struct {
	ConversationID string
	Capability     org.Capability
	Folder         string
}

// ErrNoStanding is returned when a decision would leave a capability allowed
// for good that may only ever be allowed one task at a time.
var ErrNoStanding = errors.New("that can only be allowed for one task at a time")

// Apply is what a decision on a call to capability in folder changes. An
// always-allow returns grants with the ask-mode grant that covers the call
// raised to allowed; an allow-for-task returns an Allowance, with no
// conversation, for the caller to store; allow-once and deny change nothing.
// A capability that may not be allowed standing is ErrNoStanding for an
// always-allow. The grants given are never modified.
// See docs/model.md#8-approvals.
func Apply(d Decision, grants []org.Grant, capability org.Capability, folder string) ([]org.Grant, *Allowance, error) {
	switch d {
	case AllowForTask:
		return grants, &Allowance{Capability: capability, Folder: folder}, nil
	case AlwaysAllow:
		if noStanding(capability) {
			return grants, nil, ErrNoStanding
		}
		out := slices.Clone(grants)
		for i, g := range out {
			if g.Capability == capability && g.Mode == org.ModeAsk && covers(g.Folder, folder) {
				out[i].Mode = org.ModeAllowed
			}
		}
		return out, nil, nil
	}
	return grants, nil, nil
}

// noStanding is true for a capability that has no always-allow: deleting a
// file, and asking the owner a question.
func noStanding(c org.Capability) bool {
	return c == org.FilesDelete || c == org.AskOwner
}

// covers is true when a grant scoped to granted reaches folder: it names no
// folder, or folder is it or inside it.
func covers(granted, folder string) bool {
	if granted == "" {
		return true
	}
	if folder == granted {
		return true
	}
	return strings.HasPrefix(folder, strings.TrimRight(granted, string(filepath.Separator))+string(filepath.Separator))
}
