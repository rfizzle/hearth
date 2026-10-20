package deliverables

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/deliverable/template"
)

// description is what the model reads about the tool: when to use it, the
// templates loaded for this run with what each section takes, the
// catalogue of widgets, one line each, and the rule for the office's
// figures.
// See docs/model.md#work.
func description(ts []template.Template) string {
	var b strings.Builder
	b.WriteString("Write a dashboard or a report with a fixed look: tiles, charts, tables and lists from a spec Hearth renders " +
		"as a web page beside the spec. Use it when the person asks for a dashboard, a report, a digest, a comparison or a one-page brief, " +
		"or when numbers and tables carry the message; for prose, write a Markdown file with files.write. " +
		"Then name the files it wrote in your reply.\n")
	if len(ts) > 0 {
		b.WriteString("\nPick a template and give its sections in order, with no heading or columns; each takes only the widgets it names. " +
			"With no template, build the sections yourself.\n")
		for _, t := range ts {
			fmt.Fprintf(&b, "- %s (a %s): %s Sections:", t.ID, t.Sample.Kind, t.When)
			for i, slot := range t.Slots {
				fmt.Fprintf(&b, " %d. %s, %s", i+1, t.Sample.Sections[i].Heading, kindList(slot.Allow))
				if slot.Hint != "" {
					b.WriteString(": " + slot.Hint)
				}
				if !strings.HasSuffix(b.String(), ".") {
					b.WriteString(".")
				}
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("\nBuild the sections yourself.\n")
	}
	b.WriteString("\nThe spec: {\"version\": 1, \"kind\": \"report\" or \"dashboard\", \"title\", \"subtitle\" (optional), " +
		"\"sections\": [{\"heading\", \"columns\" (1 to 3), \"widgets\": [...]}]}; at most 12 sections and 40 widgets. " +
		"Each widget has a \"kind\" and may take \"span\", the columns it covers. The widgets:\n")
	for _, line := range catalogue() {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n" + measureRule())
	return b.String()
}

// kindList is "text or callout", or "metric, progress or light".
func kindList(ks []deliverable.Kind) string {
	names := make([]string, len(ks))
	for i, k := range ks {
		names[i] = string(k)
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// catalogue is one line per widget kind, in the catalogue's order.
func catalogue() []string {
	return []string{
		`metric: "label", "value" or "measure", and optional "format" (number, integer, currency or percent), "decimals" (0 to 3), "unit", "delta" {"value", "good"} and "caption".`,
		`line: "title", "categories" (up to 52), "series" [{"name", "values", one per category}] (up to 6), and optional "format", "y_label", "y_min" and "y_max".`,
		`bar: as line, and optional "stacked".`,
		`pie: "title", "slices" [{"label", "value"}] (2 to 8, none negative), and optional "format".`,
		`table: "columns" [{"header", optional "align" (left, center or right) and "format"}] (up to 8), "rows" of cells, text or numbers, one per column (up to 100), and optional "title" and "footer".`,
		`list: "items" [{"text", optional "detail", and "done" in a check list}] (up to 50), and optional "title" and "style" (bullet, numbered or check).`,
		`progress: "label", "done" and "total", or "measure", and optional "caption".`,
		`light: "label", "tone" (neutral, positive, attention or negative) and "word", or "measure", and optional "caption".`,
		`callout: "tone", "markdown", and optional "title".`,
		`text: "markdown": headings, lists, tables and links (http, https or mailto only); no HTML and no images.`,
	}
}

// measureRule says how a figure the office keeps is bound, with an id of
// each family.
func measureRule() string {
	return "Never type a number the office keeps: bind it. A metric, a progress or a light takes \"measure\", an id, in place of its value; " +
		"Hearth fills in the value and the time it was read, and replaces any value you give. " +
		"A metric binds a number, a progress a progress, a light a light. The ids: " + strings.Join(measureIDs(), ", ") + ". " +
		"this is the project of the conversation and works only in a project's conversation; me is you; org in place of me is the whole office " +
		"(everyone for the handbook); current is this month, or give a month as yyyy-mm.\n"
}

// measureIDs is an id of each family, with its kind where it is not a
// number.
func measureIDs() []string {
	return []string{
		"project.tasks/this (a progress)", "project.light/this (a light)", "project.files/this",
		"usage.cost/me/current", "usage.tokens/me/current", "usage.budget/me/current (a progress)",
		"inbox.open", "notes.count/me", "handbook.meter/me (a progress)", "team.size",
	}
}

// parameters is the JSON Schema of the call; the template takes one of the
// ids loaded for this run.
func parameters(ts []template.Template) json.RawMessage {
	tpl := map[string]any{"type": "string", "description": "The template to fill, by its id. Leave it out to build the sections yourself."}
	if len(ts) > 0 {
		ids := make([]string, len(ts))
		for i, t := range ts {
			ids[i] = t.ID
		}
		tpl["enum"] = ids
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "The name the files take, relative to your folder and without an extension, " +
				"for example weekly-digest or reports/october. Hearth adds .deliverable.json, .html and .pdf."},
			"template": tpl,
			"spec":     map[string]any{"type": "object", "description": "The deliverable, as the description says: version, kind, title, subtitle and sections of widgets."},
		},
		"required": []string{"path", "spec"},
	}
	b, _ := json.Marshal(schema) // maps of strings and slices always marshal
	return b
}
