package agent

import (
	"encoding/json"

	"github.com/rfizzle/llm"
)

// Role is who a message is from.
type Role string

// The sides of a conversation a DM has.
const (
	RoleSystem Role = "system"
	RoleOwner  Role = "owner"
	RoleBot    Role = "bot"
	RoleTool   Role = "tool"
)

// Message is one turn of a conversation: who said it and what. A caller
// that holds earlier turns hands them in this form and never sees a
// provider's message type. A bot message that asked for tools carries the
// calls in Calls; a tool message answers the call whose id is CallID.
type Message struct {
	Role   Role
	Text   string
	Calls  []ToolCall
	CallID string
}

// ToolCall is one call a model asked for: its id, the tool and the
// arguments as JSON.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// llm is m in the provider's form.
func (m Message) llm() llm.Message {
	out := llm.Message{Role: llmRole(m.Role), Content: m.Text, ToolCallID: m.CallID}
	for _, c := range m.Calls {
		out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: c.ID, Name: c.Name, Arguments: string(c.Args)})
	}
	return out
}

// Scrub redacts what must not leave the machine from a message's text. It
// is applied to every message before it is sent and to each tool result
// before the sink sees it.
type Scrub func(Message) Message

// llmRole is the provider-side name for a role.
func llmRole(r Role) llm.Role {
	switch r {
	case RoleSystem:
		return llm.RoleSystem
	case RoleBot:
		return llm.RoleAssistant
	case RoleTool:
		return llm.RoleTool
	default:
		return llm.RoleUser
	}
}

// agentRole is the inverse of llmRole.
func agentRole(r llm.Role) Role {
	switch r {
	case llm.RoleSystem:
		return RoleSystem
	case llm.RoleAssistant:
		return RoleBot
	case llm.RoleTool:
		return RoleTool
	default:
		return RoleOwner
	}
}

// text runs s over one string as a message of the given role.
func (s Scrub) text(r llm.Role, v string) string {
	return s(Message{Role: agentRole(r), Text: v}).Text
}

// core adapts s to the loop's message type. It covers the text and the
// arguments of the calls a message made, and leaves the rest alone; nothing
// in a DM carries an attachment.
func (s Scrub) core(m llm.Message) llm.Message {
	m.Content = s.text(m.Role, m.Content)
	if len(m.ToolCalls) > 0 {
		calls := make([]llm.ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			tc.Arguments = s.text(m.Role, tc.Arguments)
			calls[i] = tc
		}
		m.ToolCalls = calls
	}
	return m
}
