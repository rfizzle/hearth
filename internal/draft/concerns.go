package draft

import (
	"encoding/json"

	"github.com/rfizzle/hearth/internal/draft/concern"
)

// concernReply is one concern as the model is asked to write it.
type concernReply struct {
	Field      string `json:"field"`
	Concern    string `json:"concern"`
	Suggestion string `json:"suggestion"`
}

// concerns is the model's concerns from the "concerns" value of a reply, as
// concern.Cap takes them: not yet capped and not yet cleaned, which Cap does.
// The concerns are advice beside a draft that is otherwise usable, so a value
// that is not a list of objects with those three text fields is no concerns,
// not a malformed reply; at most concern.Max*4 are read, since Cap serves at
// most concern.Max.
func concerns(raw json.RawMessage) []concern.Concern {
	var rs []concernReply
	if len(raw) == 0 || json.Unmarshal(raw, &rs) != nil {
		return nil
	}
	rs = rs[:min(len(rs), concern.Max*4)]
	out := make([]concern.Concern, 0, len(rs))
	for _, r := range rs {
		out = append(out, concern.Concern{Field: r.Field, Concern: r.Concern, Suggestion: r.Suggestion})
	}
	return out
}
