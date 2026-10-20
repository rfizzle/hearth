package orchestrate

import (
	"context"

	"github.com/rfizzle/hearth/internal/orchestrate/park"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// openFailure puts a run a model provider failed in the Inbox, for finish
// to publish, so the owner finds it with everything else that needs them
// and can retry or acknowledge it. An item that cannot be stored is
// logged: the reason is in the thread already. The lock is held.
// See docs/model.md#9-failure-handling.
func (k *sink) openFailure(err error) {
	it, ok := park.Failure(k.run, k.botName, err)
	if !ok {
		return
	}
	stored, err := k.s.d.Store.CreateInboxItem(k.ctx, it)
	if err != nil {
		k.s.d.Log.Printf("orchestrate: run %s failed, and its Inbox item could not be stored: %v", k.run.ID, err)
		return
	}
	k.opened = &stored
}

// superseded closes the open failure items of the runs in convID with
// outcome: a new run answers the owner, so a Retry on an older failure
// would answer their newest message instead of the one that failed.
// See docs/model.md#9-failure-handling.
func (s *Service) superseded(ctx context.Context, convID, outcome string) {
	items, err := s.d.Store.InboxItems(ctx, true)
	if err != nil {
		s.d.Log.Printf("orchestrate: the failures in %s could not be read: %v", convID, err)
		return
	}
	for _, it := range items {
		if it.Kind != inbox.Failure {
			continue
		}
		r, err := s.d.Store.Run(ctx, it.RunID)
		if err == nil && r.ConversationID == convID && s.d.Store.CloseInboxItem(ctx, it.ID, 0, "", outcome) == nil {
			_, _ = s.closed(ctx, it.ID)
		}
	}
}
