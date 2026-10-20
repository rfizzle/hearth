package wire

import (
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
)

// The bodies more than one service publishes on the bus. A body only one
// service publishes is named beside that service.

// BotChanged is a bot as it is now: new, or changed.
type BotChanged struct{ Bot org.Bot }

// NoteChanged is a note of a bot added, edited, pinned or removed. The
// orchestrator publishes it when a bot remembers or tidies, and the owner's
// edits publish the same body.
type NoteChanged = orchestrate.NoteChanged
