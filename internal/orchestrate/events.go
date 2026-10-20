package orchestrate

import "github.com/rfizzle/hearth/internal/orchestrate/bus"

// The bus and its events, under the names the service's callers use.
type (
	Bus                 = bus.Bus
	Event               = bus.Event
	MessageDelta        = bus.MessageDelta
	MessageUpserted     = bus.MessageUpserted
	MessageDeleted      = bus.MessageDeleted
	RunChanged          = bus.RunChanged
	InboxChanged        = bus.InboxChanged
	GrantsChanged       = bus.GrantsChanged
	ConversationChanged = bus.ConversationChanged
	NoteChanged         = bus.NoteChanged
	Resync              = bus.Resync
	Heartbeat           = bus.Heartbeat
)

// ProjectChanged is a project as it is now, with its derived status.
type ProjectChanged = bus.ProjectChanged

// NewBus is an empty bus whose first event is sequence 1.
func NewBus() *Bus { return bus.New() }
