package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/tools"
)

// maxText is the longest note in characters: a note is a sentence or two.
const maxText = 1000

const description = "Remember one thing for later: it is added to your notes, which you read at the start of every conversation. " +
	"Write down what will still be true and useful next week: stable facts about the owner, their preferences, decisions they made, and context that recurs. " +
	"Never write down progress on a task, what you have just done, or anything the owner can already see in the conversation or in your files. " +
	"Write one plain sentence, with the date when the date matters, and do not repeat a note you already have. " +
	"Your notes are limited; when they are full they are tidied after your reply, and the owner can pin the ones to keep."

const parameters = `{"type":"object","properties":{` +
	`"text":{"type":"string","description":"The note, in one or two plain sentences."}` +
	`},"required":["text"]}`

// Full is the reply when the note has filled the bot's notes.
const Full = "Noted; your notes are full, so they will be tidied after this reply."

// Store is the notes of one bot in one run, as the orchestrator hands them
// over: the functions are the orchestrator's, so this package does not import
// the store. ConversationID and MessageID are the link written on each note.
type Store struct {
	BotID          string
	ConversationID string
	MessageID      string
	// Add appends a note.
	Add func(ctx context.Context, n memory.Note) error
	// Count is how many notes the bot has now, pinned ones included.
	Count func(ctx context.Context, botID string) (int, error)
}

// Definition is remember, appending through s. It is allowed for good, and
// read-tier: the note is the bot's own and the owner reads and edits it.
// See docs/model.md#7-memory-writes.
func Definition(s Store) tools.Definition {
	return tools.Definition{
		Name:        string(org.Remember),
		Description: description,
		Parameters:  json.RawMessage(parameters),
		Capability:  org.Remember,
		Tier:        tools.TierRead,
		Execute:     s.execute,
		Describe:    describe,
		Validate:    validate,
	}
}

type args struct {
	Text string `json:"text"`
}

// validate refuses an empty note or one too long to be a note.
func validate(raw json.RawMessage) error {
	var a args
	if err := json.Unmarshal(raw, &a); err != nil {
		return errors.New("the arguments are not a JSON object with a text")
	}
	a.Text = memory.Plain(a.Text)
	if strings.TrimSpace(a.Text) == "" {
		return errors.New("text is required; write what to remember")
	}
	if n := utf8.RuneCountInString(a.Text); n > maxText {
		return fmt.Errorf("the note is %d characters; keep it to %d or fewer", n, maxText)
	}
	return nil
}

func (s Store) execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	if err := validate(call.Args); err != nil {
		return tools.Result{}, fmt.Errorf("remember refused: %w", err)
	}
	var a args
	_ = json.Unmarshal(call.Args, &a) // validated above
	err := s.Add(ctx, memory.Note{
		BotID: s.BotID, Text: strings.TrimSpace(memory.Plain(a.Text)),
		ConversationID: s.ConversationID, MessageID: s.MessageID,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("remember: %w", err)
	}
	n, err := s.Count(ctx, s.BotID)
	if err == nil && n >= memory.Cap {
		return tools.Result{Text: Full}, nil
	}
	return tools.Result{Text: "Noted."}, nil // the note is kept even when the count cannot be read
}

// describe is the note as the owner reads it.
func describe(raw json.RawMessage) string {
	var a args
	_ = json.Unmarshal(raw, &a) // a describer never fails
	return "Remember: " + strings.TrimSpace(memory.OneLine(memory.Plain(a.Text)))
}
