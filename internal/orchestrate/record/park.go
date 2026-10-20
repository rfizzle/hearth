package orchestrate

import (
	"errors"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/orchestrate/park"
)

// park records the call the run stopped on: the request in the thread and
// the item in the Inbox, kept for finish to publish once the conversation
// is free. A request whose item could not be stored is taken out of the
// thread again, so no card waits on nothing. The lock is held.
// See docs/model.md#8-approvals.
func (k *sink) park(c *agent.ToolCall) error {
	if c == nil {
		return errors.New("the run stopped to wait for you, but on nothing")
	}
	// The card is described from the parked call's own arguments, with the
	// path the dispatcher resolved, so neither a path through .. or a link
	// nor a call id the model reused can put other words on it.
	g := park.Call{ID: c.ID, Name: c.Name, Args: c.Args}
	if checked, err := k.disp.Check(c.Name, c.Args); err == nil {
		g.Effect = checked.Describe()
	}
	if g.Effect == "" {
		g.Effect = "Use " + c.Name
	}
	msg, item := park.Request(k.run, g)
	m, ok := k.append(msg)
	if !ok {
		return k.err
	}
	item.MessageID = m.ID
	stored, err := k.s.d.Store.CreateInboxItem(k.ctx, item)
	if err != nil {
		if derr := k.s.d.Store.DeleteMessage(k.ctx, m.ID); derr == nil {
			k.s.d.Bus.Publish(MessageDeleted{ConversationID: m.ConversationID, MessageID: m.ID})
		}
		return err
	}
	k.opened = &stored
	return nil
}
