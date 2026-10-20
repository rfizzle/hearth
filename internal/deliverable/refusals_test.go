package deliverable

import (
	"errors"
	"strings"
	"testing"
)

// doc wraps a widget in the smallest report.
func doc(widget string) string {
	return `{"version":1,"kind":"report","title":"T","sections":[{"widgets":[` + widget + `]}]}`
}

func rep(n int, s string) string { return strings.Repeat(s, n) }

const (
	textW = `{"kind":"text","markdown":"x"}`
	head  = `{"version":1,"kind":"report","title":"T","sections":`
)

// refusals is one row per refusal of the catalogue, with the words it says. A
// spec reaches a row through Parse, then Validate.
var refusals = []struct{ name, in, want string }{
	{"not json", `{"version":`, "the spec is not valid JSON"},
	{"text after the json", `{} {}`, "the spec is not valid JSON"},
	{"a null", `{"version":null}`, "the spec has a null"},
	{"too deep", rep(11, "[") + rep(11, "]"), "nests more than 10 levels"},
	{"over the byte cap", `{"title":"` + rep(MaxBytes, "a") + `"}`, "the limit is 256 KB"},
	{"unknown top field", `{"version":1,"colour":"red"}`, `the field "colour" is not one a deliverable takes`},
	{"unknown section field", head + `[{"colour":1,"widgets":[]}]}`, `section 1: the field "colour" is not one a section takes`},
	{"unknown widget field", doc(`{"kind":"text","markdown":"x","colour":"red"}`), `section 1, widget 1: the field "colour" is not one a text takes`},
	{"a field of another kind", doc(`{"kind":"metric","label":"L","value":1,"categories":["a"]}`), `the field "categories" is not one a metric takes`},
	{"unknown nested field", doc(`{"kind":"pie","title":"P","slices":[{"label":"a","value":1,"x":2}]}`), `is not one a pie takes`},
	{"version", `{"version":2,"kind":"report","title":"T","sections":[]}`, "this Hearth writes version 1 deliverables; set version to 1"},
	{"missing kind", `{"version":1,"title":"T","sections":[]}`, "kind is report or dashboard"},
	{"unknown kind", `{"version":1,"kind":"poster","title":"T","sections":[]}`, "kind is report or dashboard"},
	{"unknown widget kind", doc(`{"kind":"gauge"}`), `the kind "gauge" is not one of the widgets; use one of: metric, line, bar, pie, table, list, progress, light, callout, text`},
	{"no widget kind", doc(`{}`), `the kind "" is not one of the widgets`},
	{"empty title", `{"version":1,"kind":"report","title":"","sections":[{"widgets":[` + textW + `]}]}`, "the title is empty; give it 1 to 120 characters"},
	{"long title", `{"version":1,"kind":"report","title":"` + rep(121, "a") + `","sections":[]}`, "the title has 121 characters; the limit is 120"},
	{"control in title", `{"version":1,"kind":"report","title":"a\nb","sections":[]}`, "the title holds a line break or another control character; write it on one line"},
	{"no sections", head + `[]}`, "the spec has 0 sections; give 1 to 12"},
	{"too many sections", head + `[` + rep(12, `{"widgets":[`+textW+`]},`) + `{"widgets":[` + textW + `]}]}`, "the spec has 13 sections; the limit is 12"},
	{"long heading", head + `[{"heading":"` + rep(81, "h") + `","widgets":[` + textW + `]}]}`, "section 1: the heading has 81 characters; the limit is 80"},
	{"columns", head + `[{"columns":4,"widgets":[` + textW + `]}]}`, "section 1: columns go from 1 to 3; you gave 4"},
	{"no widgets", head + `[{"widgets":[]}]}`, "section 1: the section has 0 widgets; give 1 to 12"},
	{"too many in a section", doc(rep(12, textW+",") + textW), "section 1: the section has 13 widgets; the limit is 12"},
	{"too many in all", head + `[` + rep(4, `{"widgets":[`+rep(11, textW+",")+textW+`]},`) + `{"widgets":[` + textW + `]}]}`, "the spec has 49 widgets; the limit is 40"},
	{"span", doc(`{"kind":"text","markdown":"x","span":2}`), "section 1, widget 1: the widget spans 2 columns; the section has 1"},
	{"a number as text", doc(`{"kind":"metric","label":"L","value":"12"}`), "values are numbers, not text"},
	{"a series value as text", doc(`{"kind":"line","title":"T","categories":["a"],"series":[{"name":"s","values":["1"]}]}`), "values are numbers, not text"},
	{"a number too large to read", doc(`{"kind":"metric","label":"L","value":1e999}`), "the value 1e999 is too large; numbers go up to 15 digits"},
	{"a number too large", doc(`{"kind":"metric","label":"L","value":1e300}`), "the value 1e+300 is too large; numbers go up to 15 digits"},
	{"text for a number field", doc(`{"kind":"metric","label":5,"value":1}`), `the field "label" wants text`},
	{"a widget that is not an object", doc(`"metric"`), "the widget wants an object, not string"},
	{"measure on a table", doc(`{"kind":"table","columns":[{"header":"a"}],"rows":[["x"]],"measure":"team.size"}`), "a table cannot bind a measure; only metric, progress and light take one"},
	{"measure with a bad character", doc(`{"kind":"metric","label":"L","measure":"team size"}`), `the measure "team size" holds a character an id does not use`},
	{"as_of with no measure", doc(`{"kind":"metric","label":"L","value":1,"as_of":"2026-10-20T09:30Z"}`), "as_of belongs to a bound widget, and this one has no measure"},
	{"as_of badly written", doc(`{"kind":"metric","label":"L","measure":"team.size","as_of":"yesterday"}`), `as_of is a time in UTC to the minute, like 2026-10-20T09:30Z; you gave "yesterday"`},
	{"metric with no value", doc(`{"kind":"metric","label":"L"}`), "a metric needs a value or a measure"},
	{"metric label", doc(`{"kind":"metric","label":"","value":1}`), "the label is empty; give it 1 to 40 characters"},
	{"metric format", doc(`{"kind":"metric","label":"L","value":1,"format":"roman"}`), `format is number, integer, currency or percent; you gave "roman"`},
	{"metric decimals", doc(`{"kind":"metric","label":"L","value":1,"decimals":4}`), "decimals go from 0 to 3; you gave 4"},
	{"metric unit", doc(`{"kind":"metric","label":"L","value":1,"unit":"` + rep(13, "u") + `"}`), "the unit has 13 characters; the limit is 12"},
	{"control in a label", doc(`{"kind":"metric","label":"a\tb","value":1}`), "section 1, widget 1: the label holds a line break or another control character; write it on one line"},
	{"series short", doc(`{"kind":"line","title":"T","categories":["a","b","c","d"],"series":[{"name":"Dana","values":[1,2,3]}]}`), `series "Dana" has 3 values for 4 categories; give one value per category`},
	{"series long", doc(`{"kind":"bar","title":"T","categories":["a"],"series":[{"name":"Dana","values":[1,2]}]}`), `series "Dana" has 2 values for 1 categories`},
	{"too many categories", doc(`{"kind":"line","title":"T","categories":[` + rep(52, `"a",`) + `"a"],"series":[{"name":"s","values":[1]}]}`), "the chart has 53 categories; the limit is 52"},
	{"too many series", doc(`{"kind":"line","title":"T","categories":["a"],"series":[` + rep(6, `{"name":"s","values":[1]},`) + `{"name":"s","values":[1]}]}`), "the chart has 7 series; the limit is 6"},
	{"no series", doc(`{"kind":"line","title":"T","categories":["a"],"series":[]}`), "the chart has 0 series; give 1 to 6"},
	{"long category", doc(`{"kind":"line","title":"T","categories":["` + rep(25, "c") + `"],"series":[{"name":"s","values":[1]}]}`), "category 1 has 25 characters; the limit is 24"},
	{"chart title", doc(`{"kind":"line","categories":["a"],"series":[{"name":"s","values":[1]}]}`), "the title is empty; give it 1 to 80 characters"},
	{"y range", doc(`{"kind":"line","title":"T","categories":["a"],"series":[{"name":"s","values":[1]}],"y_min":5,"y_max":5}`), "y_min 5 is not below y_max 5"},
	{"stacked on a line", doc(`{"kind":"line","title":"T","categories":["a"],"series":[{"name":"s","values":[1]}],"stacked":true}`), `the field "stacked" is not one a line takes`},
	{"negative slice", doc(`{"kind":"pie","title":"P","slices":[{"label":"Dana","value":2},{"label":"Sam","value":-1}]}`), `slice "Sam" is negative; a pie takes zero or more`},
	{"zero pie", doc(`{"kind":"pie","title":"P","slices":[{"label":"Dana","value":0},{"label":"Sam","value":0}]}`), "every slice is zero; a pie needs something to show"},
	{"one slice", doc(`{"kind":"pie","title":"P","slices":[{"label":"Dana","value":1}]}`), "the pie has 1 slices; give 2 to 8"},
	{"too many rows", doc(`{"kind":"table","columns":[{"header":"a"}],"rows":[` + rep(100, `["x"],`) + `["x"]]}`), "the table has 101 rows; the limit is 100"},
	{"too many columns", doc(`{"kind":"table","columns":[` + rep(8, `{"header":"a"},`) + `{"header":"a"}],"rows":[["x"]]}`), "the table has 9 columns; the limit is 8"},
	{"ragged row", doc(`{"kind":"table","columns":[{"header":"a"},{"header":"b"}],"rows":[["x","y"],["x"]]}`), "row 2 has 1 cells for 2 columns; give one cell per column"},
	{"ragged footer", doc(`{"kind":"table","columns":[{"header":"a"},{"header":"b"}],"rows":[["x","y"]],"footer":["t"]}`), "the footer has 1 cells for 2 columns"},
	{"a cell that is neither", doc(`{"kind":"table","columns":[{"header":"a"}],"rows":[[true]]}`), "cell 1 of row 1 is text or a number"},
	{"a long cell", doc(`{"kind":"table","columns":[{"header":"a"}],"rows":[["` + rep(121, "c") + `"]]}`), "cell 1 of row 1 has 121 characters; the limit is 120"},
	{"column align", doc(`{"kind":"table","columns":[{"header":"a","align":"up"}],"rows":[["x"]]}`), `align is left, center or right; you gave "up"`},
	{"too many items", doc(`{"kind":"list","items":[` + rep(50, `{"text":"a"},`) + `{"text":"a"}]}`), "the list has 51 items; the limit is 50"},
	{"list style", doc(`{"kind":"list","style":"fancy","items":[{"text":"a"}]}`), `style is bullet, numbered or check; you gave "fancy"`},
	{"done outside a check list", doc(`{"kind":"list","items":[{"text":"a","done":true}]}`), "item 1 has done but the list is not a check list; set style to check"},
	{"progress over total", doc(`{"kind":"progress","label":"L","done":8,"total":7}`), "done 8 is more than total 7"},
	{"progress below zero", doc(`{"kind":"progress","label":"L","done":-1,"total":7}`), "done -1 is below zero"},
	{"progress total too large", doc(`{"kind":"progress","label":"L","done":1,"total":2e9}`), "total 2000000000 is too large; the limit is 1000000000"},
	{"progress with nothing", doc(`{"kind":"progress","label":"L"}`), "a progress widget needs done and total, or a measure"},
	{"progress with one", doc(`{"kind":"progress","label":"L","done":1}`), "give both done and total, or neither"},
	{"light with nothing", doc(`{"kind":"light","label":"L"}`), "a light needs a tone and a word, or a measure"},
	{"light word without tone", doc(`{"kind":"light","label":"L","word":"Fine"}`), "give tone and word together; the word always travels with the tone"},
	{"light tone", doc(`{"kind":"light","label":"L","tone":"red","word":"Fine"}`), `tone is neutral, positive, attention or negative; you gave "red"`},
	{"light long word", doc(`{"kind":"light","label":"L","tone":"neutral","word":"` + rep(25, "w") + `"}`), "the word has 25 characters; the limit is 24"},
	{"callout without a tone", doc(`{"kind":"callout","markdown":"x"}`), `tone is neutral, positive, attention or negative; you gave ""`},
	{"callout too long", doc(`{"kind":"callout","tone":"neutral","markdown":"` + rep(2001, "a") + `"}`), "the callout has 2001 characters; the limit is 2000"},
	{"empty text", doc(`{"kind":"text","markdown":""}`), "the text is empty; give it 1 to 20000 characters"},
	{"text too long", doc(`{"kind":"text","markdown":"` + rep(20001, "a") + `"}`), "the text has 20001 characters; the limit is 20000"},
	{"control in markdown", doc(`{"kind":"text","markdown":"a\u0000b"}`), "the text holds a control character other than line breaks and tabs; remove it"},
	{"html block", doc(`{"kind":"text","markdown":"<div>hi</div>"}`), "section 1, widget 1: the text has HTML in it (`<div>hi</div>`); write Markdown only"},
	{"inline html", doc(`{"kind":"callout","tone":"neutral","markdown":"a <b>bold</b> b"}`), "the callout has HTML in it (`<b>`); write Markdown only"},
	{"a script", doc(`{"kind":"text","markdown":"<script>alert(1)</script>"}`), "the text has HTML in it"},
	{"an image", doc(`{"kind":"text","markdown":"![a cat](https://example.com/cat.png)"}`), "images are not part of a shaped deliverable yet; describe it or put it in a Markdown file"},
	{"an image inside a link", doc(`{"kind":"text","markdown":"[![a](https://x.test/a.png)](https://x.test)"}`), "images are not part of a shaped deliverable yet"},
	{"javascript link", doc(`{"kind":"text","markdown":"[x](javascript:alert(1))"}`), `the link "javascript:alert(1)" is not a web address; links are http, https or mailto`},
	{"file link", doc(`{"kind":"text","markdown":"[x](file:///etc/passwd)"}`), "is not a web address"},
	{"data link", doc(`{"kind":"text","markdown":"[x](data:text/html;base64,AAAA)"}`), "is not a web address"},
	{"a bare scheme", doc(`{"kind":"text","markdown":"[x](https:)"}`), "is not a web address"},
	{"a relative link", doc(`{"kind":"text","markdown":"[x](../secret)"}`), "is not a web address"},
	{"a scheme in capitals", doc(`{"kind":"text","markdown":"[x](JaVaScRiPt:alert(1))"}`), "is not a web address"},
	{"a scheme spelled with an entity", doc(`{"kind":"text","markdown":"[x](&#106;avascript:alert(1))"}`), "is not a web address"},
	{"a scheme with a tab", doc(`{"kind":"text","markdown":"[x](<java\tscript:alert(1)>)"}`), "is not a web address"},
	{"an autolink to javascript", doc(`{"kind":"text","markdown":"<javascript:alert(1)>"}`), "is not a web address"},
	{"a reference link", doc(`{"kind":"text","markdown":"[x][r]\n\n[r]: javascript:alert(1)"}`), "is not a web address"},
}

func TestRefusals(t *testing.T) {
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Parse([]byte(tc.in))
			if err == nil {
				err = Validate(s)
			}
			if err == nil {
				t.Fatalf("accepted; want %q", tc.want)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("%v does not wrap ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q; want it to say %q", err, tc.want)
			}
		})
	}
}
