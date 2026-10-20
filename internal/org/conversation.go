package org

import "time"

// ConversationKind is what a conversation is for.
type ConversationKind int

// ConversationDM is a bot's direct conversation with the owner.
const ConversationDM ConversationKind = 1

// Conversation is one thread. A DM belongs to the bot in BotID. LatestRun is
// the newest run in it, in whatever state, when a list fills it in, and nil
// before the first message.
type Conversation struct {
	ID        string
	Kind      ConversationKind
	BotID     string
	Title     string
	LatestRun *Run
	CreatedAt time.Time
	UpdatedAt time.Time

	// Summary stands for the messages up to SummaryThroughSeq, which the
	// prompt no longer carries whole.
	Summary           string
	SummaryThroughSeq int64
}

// MessageKind is what a message in a thread is.
type MessageKind int

// The kinds of message a DM holds.
const (
	MessageText             MessageKind = 1
	MessageToolCall         MessageKind = 2
	MessageToolResult       MessageKind = 3
	MessageQuestion         MessageKind = 6
	MessageApprovalRequest  MessageKind = 7
	MessageApprovalDecision MessageKind = 8
	MessageSystem           MessageKind = 9
)

// Message is one row of a thread. Seq orders it within its conversation and
// is assigned when it is stored. AuthorBotID is empty for the owner. A tool
// call message carries the calls the model made in ToolCalls; a tool result
// carries the call it answers in ToolCallID. Streaming is true while the
// text is still arriving. Widget is the JSON of the control a question asks
// with and WidgetAnswer the JSON of the owner's answer to it; both are empty
// for a message with none.
type Message struct {
	ID             string
	ConversationID string
	Seq            int64
	AuthorBotID    string
	Kind           MessageKind
	Content        string
	ToolCalls      []ToolCall
	ToolCallID     string
	Files          []FileRef
	Attachments    []FileRef
	RunID          string
	Streaming      bool
	Widget         string
	WidgetAnswer   string
	CreatedAt      time.Time
}

// ToolCall is one call a model made: its id, the tool, the arguments as JSON
// and a plain-words summary for the owner.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Summary   string `json:"summary,omitempty"`
}

// FileRef is a file a message points at: its absolute path and the name to
// show.
type FileRef struct {
	Path string `json:"path"`
	Name string `json:"name"`
}
