package framing_test

import (
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/framing"
)

// concernsParagraph is the text both drafts end with.
const concernsParagraph = `Then, in a "concerns" list of at most three items, say what in the person's words would make the work hard to do well, each as {"field": one of the field names above, "concern": one plain sentence, "suggestion": a replacement for that field, or an empty string}. An empty list is a fine answer.`

func TestDrafts_EndWithTheConcernsParagraph(t *testing.T) {
	for name, got := range map[string]string{
		"hire":    framing.RenderDraft(framing.DraftInput{}),
		"project": framing.RenderDraftProject(),
	} {
		if !strings.HasSuffix(strings.TrimSpace(got), concernsParagraph) {
			t.Errorf("%s draft text does not end with the concerns paragraph:\n%s", name, got)
		}
	}
}
