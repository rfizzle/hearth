package orchestrate

import (
	"context"

	"github.com/rfizzle/hearth/internal/org/inbox"
)

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
