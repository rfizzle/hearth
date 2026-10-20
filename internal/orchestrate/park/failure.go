package park

import (
	"errors"
	"strings"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// providerFailure is the method set of a model provider's classified
// failure, matched by its methods since only the agent and the provider
// import the provider library.
type providerFailure interface {
	error
	Headline() string
	Recoverable() bool
}

// Failure is the Inbox item the run r of botName's leaves when a model
// provider failed it, and false for a run that ended any other way: a cap, a
// stop or a failure of hearth's own leaves its reason in the thread alone.
// The item names the run, for Retry, and holds no conversation, so the owner
// can still write to the bot while it is open.
// See docs/model.md#9-failure-handling.
func Failure(r org.Run, botName string, err error) (inbox.Item, bool) {
	var f providerFailure
	if !errors.As(err, &f) {
		return inbox.Item{}, false
	}
	reason := agent.Explain(err)
	title := botName + ": " + reason
	if strings.HasPrefix(reason, "could not ") {
		title = botName + " " + reason
	}
	return inbox.Item{Kind: inbox.Failure, BotID: r.BotID, RunID: r.ID, Title: title, Effect: reason}, true
}
