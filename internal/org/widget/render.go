package widget

import (
	"fmt"
	"strings"
)

// Render turns an answer into the fixed words the bot reads. The Spec supplies
// the labels; an id it does not know is shown as it is. An empty answer
// renders as nothing, except a multiple choice, which reads "none".
func Render(s Spec, a Answer) string {
	switch s.Kind {
	case SingleChoice:
		if s.Single == nil || a.empty() {
			return ""
		}
		return "The owner chose: " + chosen(s.Single.Options, a)
	case MultipleChoice:
		if s.Multiple == nil || a.empty() {
			return "The owner chose none"
		}
		return "The owner chose: " + chosen(s.Multiple.Options, a)
	case FreeText:
		if a.Text == "" {
			return ""
		}
		return "The owner answered: " + a.Text
	case YesNo:
		return renderYesNo(s.YesNo, a)
	case Date:
		if a.Date == "" {
			return ""
		}
		return "The owner picked the date: " + a.Date
	case Path:
		if a.Path == "" {
			return ""
		}
		return "The owner picked: " + a.Path
	case Scale:
		if a.Value == nil || s.Scale == nil {
			return ""
		}
		return fmt.Sprintf("The owner rated it %d of %d", *a.Value, s.Scale.Max)
	}
	return ""
}

// chosen joins the labels of the chosen options, then Other.
func chosen(opts []Option, a Answer) string {
	parts := make([]string, 0, len(a.Choices)+1)
	for _, id := range a.Choices {
		label := id
		for _, o := range opts {
			if o.ID == id {
				label = o.Label
			}
		}
		parts = append(parts, label)
	}
	if a.Other != "" {
		parts = append(parts, "Other: "+a.Other)
	}
	return strings.Join(parts, ", ")
}

func renderYesNo(y *YesNoSpec, a Answer) string {
	if a.Yes == nil {
		return ""
	}
	yes, no := "Yes", "No"
	if y != nil {
		if y.YesLabel != "" {
			yes = y.YesLabel
		}
		if y.NoLabel != "" {
			no = y.NoLabel
		}
	}
	if *a.Yes {
		return "The owner said: " + yes
	}
	return "The owner said: " + no
}
