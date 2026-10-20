package assembly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/org/memory"
)

// ErrMalformed is a reply that was not the list of notes that was asked for.
// The notes are left as they were, and are tried again after the next run.
var ErrMalformed = errors.New("the tidied notes came back in a form that could not be read")

// Notes is the bot's notes as Consolidate reads and replaces them; the
// store's *DB is one.
type Notes interface {
	Notes(ctx context.Context, botID string) ([]memory.Note, error)
	ReplaceNotes(ctx context.Context, botID string, unpinned []memory.Note) error
}

// Consolidate asks the model, once and with no tools, to merge the bot's
// unpinned notes into fewer, and replaces them with the answer in one step;
// the pinned notes are shown as fixed and never change. It returns how many
// notes the bot had and has, pinned ones counted. The reply must be a JSON
// list of non-empty strings, no more than the notes it was given and no more
// than framing.ConsolidateMax; anything else is ErrMalformed and changes
// nothing, as does a model that cannot be reached. A bot with no unpinned
// notes is left alone. A note written between the read and the replace is
// replaced with the rest, so the caller runs it when the bot is not. The
// request's usage goes to side.Usage.
// See docs/model.md#7-memory-writes.
func Consolidate(ctx context.Context, db Notes, side Side, botID string) (before, after int, err error) {
	all, err := db.Notes(ctx, botID)
	if err != nil {
		return 0, 0, fmt.Errorf("read notes: %w", err)
	}
	var pinned, unpinned []memory.Note
	for _, n := range all {
		if n.Pinned {
			pinned = append(pinned, n)
		} else {
			unpinned = append(unpinned, n)
		}
	}
	before = len(all)
	if len(unpinned) == 0 {
		return before, before, nil
	}
	// Text the bot wrote (its notes) enters the prompt here, only inside the
	// labelled lists of consolidate.md.
	reply, err := side.ask(ctx, framing.RenderConsolidate(pinned, unpinned), "Tidy the notes.")
	if err != nil {
		return before, before, fmt.Errorf("tidy notes: %s", agent.Explain(err))
	}
	texts, err := parseNotes(reply, min(len(unpinned), framing.ConsolidateMax(len(pinned))))
	if err != nil {
		return before, before, err
	}
	tidied := make([]memory.Note, len(texts))
	for i, t := range texts {
		tidied[i] = memory.Note{Text: t}
	}
	if err := db.ReplaceNotes(ctx, botID, tidied); err != nil {
		return before, before, fmt.Errorf("replace notes: %w", err)
	}
	return before, len(pinned) + len(tidied), nil
}

// parseNotes reads the model's text defensively: the list runs from the
// first opening bracket to the last closing one, so a fence or a sentence
// around it does not matter. Each note is trimmed; an empty note, an empty
// list or one longer than limit is ErrMalformed.
func parseNotes(text string, limit int) ([]string, error) {
	start, end := strings.Index(text, "["), strings.LastIndex(text, "]")
	if start < 0 || end < start {
		return nil, ErrMalformed
	}
	var texts []string
	if err := json.Unmarshal([]byte(text[start:end+1]), &texts); err != nil {
		return nil, ErrMalformed
	}
	if len(texts) == 0 || len(texts) > limit {
		return nil, ErrMalformed
	}
	for i, t := range texts {
		if texts[i] = strings.TrimSpace(t); texts[i] == "" {
			return nil, ErrMalformed
		}
	}
	return texts, nil
}
