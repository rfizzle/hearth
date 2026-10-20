package draft_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/org"
)

// A service that answers with something that is not a reply ends the stream
// with an error naming it; the draft says so rather than that the reply
// could not be read.
func TestDraft_ServiceThatSendsNoReply(t *testing.T) {
	sent := errors.New(`Acme gateway answered, but not with a reply (it sent {"Output":{"__type":"com.amazon.coral.service#UnknownOperationException"}). Check the address and the model name`)
	stream := func(context.Context, []llm.Message, []llm.Tool, string) (<-chan llm.StreamEvent, error) {
		ch := make(chan llm.StreamEvent, 1)
		ch <- llm.StreamEvent{Done: true, Err: sent}
		close(ch)
		return ch, nil
	}
	_, err := draft.Draft(t.Context(), draft.Input{Description: "a researcher", Tier: org.TierFast, Stream: stream})
	if !errors.Is(err, draft.ErrUnavailable) || errors.Is(err, draft.ErrMalformed) || !strings.Contains(err.Error(), "Acme gateway answered") {
		t.Errorf("Draft error = %v, want the unavailable error carrying the service's words", err)
	}
}
