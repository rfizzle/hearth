package record

import "github.com/rfizzle/hearth/internal/orchestrate/park"

// openFailure puts a run a model provider failed in the Inbox, for finish
// to publish, so the owner finds it with everything else that needs them
// and can retry or acknowledge it. An item that cannot be stored is
// logged: the reason is in the thread already. The lock is held.
// See docs/model.md#9-failure-handling.
func (k *Sink) openFailure(err error) {
	it, ok := park.Failure(k.run, k.botName, err)
	if !ok {
		return
	}
	stored, err := k.h.Store.CreateInboxItem(k.ctx, it)
	if err != nil {
		k.h.Log.Printf("orchestrate: run %s failed, and its Inbox item could not be stored: %v", k.run.ID, err)
		return
	}
	k.opened = &stored
}
