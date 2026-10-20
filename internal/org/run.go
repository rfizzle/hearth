package org

import "time"

// RunState is where a run is in its life.
type RunState int

// The states a run moves through. Done, failed and cancelled are terminal; a
// run waiting on the owner holds no model call and ends when they answer.
const (
	RunQueued       RunState = 1
	RunWorking      RunState = 2
	RunWaitingOwner RunState = 3
	RunDone         RunState = 4
	RunFailed       RunState = 5
	RunCancelled    RunState = 6
)

// Run is one bot answering one message. Steps counts its tool rounds and
// Failure says in plain words why a failed run stopped. StartedAt and EndedAt
// are zero until the run starts and ends.
type Run struct {
	ID             string
	ConversationID string
	BotID          string
	State          RunState
	Steps          int
	Failure        string
	StartedAt      time.Time
	EndedAt        time.Time
}

// Caps are the limits a run ends at whatever the model does: a number of tool
// rounds, a round being one model reply's tool calls, and a span of wall time.
type Caps struct {
	Steps int
	Wall  time.Duration
}

// DefaultCaps are 25 tool rounds and 10 minutes, so a run always ends.
// See docs/model.md#4-termination-guarantees.
func DefaultCaps() Caps {
	return Caps{Steps: 25, Wall: 10 * time.Minute}
}
