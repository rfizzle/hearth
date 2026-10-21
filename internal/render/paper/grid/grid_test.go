package grid

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/deliverable"
	shipped "github.com/rfizzle/hearth/internal/deliverable/template"
	"github.com/rfizzle/hearth/internal/document/pdftext"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// update rewrites the golden: go test ./internal/render/paper/grid -run TestGolden -update
//
// The golden is the PDF's bytes, which pass through compress/flate: a Go
// toolchain upgrade may change its output without any change here. When the
// golden fails after a toolchain bump and nothing else, re-bless it with
// -update in the same commit as the bump and say so.
var update = flag.Bool("update", false, "rewrite the golden file under testdata")

var modTime = time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC)

// kitchenSink is the web page's every-widget spec, read where that package
// keeps it, so the two renderers are proved on the same input.
func kitchenSink(t *testing.T) deliverable.Spec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "html", "testdata", "kitchen-sink.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := deliverable.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := deliverable.Validate(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func render(t *testing.T, s deliverable.Spec, o Options) ([]byte, paper.Report) {
	t.Helper()
	out, rep, err := Render(context.Background(), s, o)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if rep.Bytes != len(out) || rep.Pages < 1 || !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("Report %+v for %d bytes", rep, len(out))
	}
	return out, rep
}

// pages reads every page of a PDF back through the document reader, its
// words joined by single spaces.
func pages(t *testing.T, pdf []byte) []string {
	t.Helper()
	r, err := pdftext.Open(context.Background(), bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatalf("reading the PDF back: %v", err)
	}
	var out []string
	for n := 1; n <= r.NumPage(); n++ {
		p, err := r.Page(context.Background(), n)
		if err != nil {
			t.Fatalf("page %d: %v", n, err)
		}
		out = append(out, strings.Join(strings.Fields(p), " "))
	}
	return out
}

// inOrder reports the first of want not found in text after the one before.
func inOrder(text string, want []string) (string, bool) {
	at := 0
	for _, w := range want {
		i := strings.Index(text[at:], w)
		if i < 0 {
			return w, false
		}
		at += i + len(w)
	}
	return "", true
}

func TestRenderIsReproducible(t *testing.T) {
	s := kitchenSink(t)
	first, rep := render(t, s, Options{ModTime: modTime})
	second, _ := render(t, s, Options{ModTime: modTime})
	if !bytes.Equal(first, second) {
		t.Fatal("the same spec rendered twice gave different bytes")
	}
	for _, want := range []string{"/Producer (Hearth)", "/CreationDate (D:20261020093000)", "/ModDate (D:20261020093000)", "/MediaBox [0 0 792.00 612.00]"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("the dashboard's PDF is missing %s", want)
		}
	}
	if rep.Pages >= MaxPages {
		t.Errorf("the kitchen sink renders to %d pages", rep.Pages)
	}
}

func TestGolden(t *testing.T) {
	got, _ := render(t, kitchenSink(t), Options{ModTime: modTime})
	path := filepath.Join("testdata", "kitchen-sink.pdf")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the kitchen sink's PDF differs from %s (%d bytes, want %d); if only the Go toolchain changed, re-bless with -update", path, len(got), len(want))
	}
}

// TestTextInOrder reads the kitchen sink back: the title, every heading,
// every label and every table cell, in the order the spec gives them.
func TestTextInOrder(t *testing.T) {
	out, _ := render(t, kitchenSink(t), Options{ModTime: modTime})
	text := strings.Join(pages(t, out), " ")
	want := []string{
		"Office this week", "Week of 13 October 2026 & what comes next",
		"The numbers", "Spent this month", "$12.40", "-$3.10", "Budget $40", "As of 2026-10-20 09:30 UTC",
		"A label of exactly forty characters long", "Tokens", "Ratio", "3.14", "Not read yet",
		"Launch the API tier", "3 of 7", "Nothing to do", "API tier launch", "Waiting on you", "Venue", "On track", "Printing", "Stopped", "Archive", "Idle",
		"Trends", "Cost per week", "W40", "Dana", "Tasks by state", "Done", "Spend by employee",
		"Open items", "Open items", "Item", "Owner", "Cost", "Share", "Count", "Launch email", "Dana", "$12.50", "40.0%", "3",
		"Total", "$1,262.50",
		"Bullets", "First", "Second", "With a detail", "Steps", "1.", "Draft", "2.", "Review", "Next week", "Send the launch email", "Book the venue", "Confirmed Tuesday",
		"Notes", "Needs you", "Two tasks are waiting", "Open the Inbox", "Done", "The venue is booked.", "Printing stopped.", "For the record",
		"Heading one reads as two", "Two", "Three", "a done task", "a quote", "Left", "Centre", "Right", "Write to Dana",
	}
	if miss, ok := inOrder(text, want); !ok {
		t.Errorf("%q is missing or out of order in:\n%s", miss, text)
	}
}

// errands is a report of one table with n rows.
func errands(n int) deliverable.Spec {
	rows := make([][]any, n)
	for i := range rows {
		rows[i] = []any{fmt.Sprintf("errand %03d", i+1), float64(i + 1)}
	}
	return deliverable.Spec{Version: 1, Kind: deliverable.Report, Title: "Errands", Sections: []deliverable.Section{{Widgets: []deliverable.Widget{{
		Kind: deliverable.Table, Title: "All errands",
		Columns: []deliverable.Column{{Header: "Errand"}, {Header: "Order", Align: "right", Format: "integer"}},
		Rows:    rows,
	}}}}}
}

func TestTableContinuesWithItsHeader(t *testing.T) {
	s := errands(100)
	if err := deliverable.Validate(s); err != nil {
		t.Fatal(err)
	}
	out, rep := render(t, s, Options{ModTime: modTime})
	if !bytes.Contains(out, []byte("/MediaBox [0 0 612.00 792.00]")) {
		t.Error("a report is not portrait Letter")
	}
	ps := pages(t, out)
	if rep.Pages < 2 || len(ps) != rep.Pages {
		t.Fatalf("100 rows took %d pages (%d read)", rep.Pages, len(ps))
	}
	var rows []string
	for i, p := range ps {
		if !strings.Contains(p, "Errand Order") {
			t.Errorf("page %d has no header row: %q", i+1, p)
		}
		rows = append(rows, p)
	}
	all := strings.Join(rows, " ")
	for i := 1; i <= 100; i++ {
		row := fmt.Sprintf("errand %03d %d", i, i)
		if !strings.Contains(all, row) {
			t.Fatalf("row %q is missing", row)
		}
	}
}

// TestPageCap: forty long tables run past sixty pages and are refused in
// words, with nothing drawn.
func TestPageCap(t *testing.T) {
	long := errands(100).Sections[0].Widgets[0]
	s := deliverable.Spec{Version: 1, Kind: deliverable.Report, Title: "Too long"}
	for range 4 {
		sec := deliverable.Section{}
		for range 10 {
			sec.Widgets = append(sec.Widgets, long)
		}
		s.Sections = append(s.Sections, sec)
	}
	if err := deliverable.Validate(s); err != nil {
		t.Fatal(err)
	}
	out, _, err := Render(context.Background(), s, Options{})
	if !errors.Is(err, ErrTooLong) || out != nil {
		t.Fatalf("forty tables = %d bytes, %v; want ErrTooLong", len(out), err)
	}
	if err.Error() != "the deliverable renders to more than 60 pages; shorten the tables" {
		t.Errorf("the refusal reads %q", err)
	}
}

// TestSamples renders each shipped template's sample on both papers.
func TestSamples(t *testing.T) {
	for _, tpl := range shipped.Shipped() {
		for _, sheet := range []paper.Paper{paper.Letter, paper.A4} {
			out, rep := render(t, tpl.Sample, Options{Paper: sheet, ModTime: modTime})
			if rep.Pages > 4 {
				t.Errorf("%s on %s took %d pages", tpl.ID, sheet, rep.Pages)
			}
			text := strings.Join(pages(t, out), " ")
			if !strings.Contains(text, tpl.Sample.Title) {
				t.Errorf("%s on %s does not read back its title", tpl.ID, sheet)
			}
		}
	}
}

// TestTallCard: a text longer than a page in a two-column grid is drawn
// alone and continued, and the cards after it still draw.
func TestTallCard(t *testing.T) {
	var b strings.Builder
	for i := range 120 {
		fmt.Fprintf(&b, "Paragraph %d of the long note.\n\n", i+1)
	}
	s := deliverable.Spec{Version: 1, Kind: deliverable.Dashboard, Title: "Tall", Sections: []deliverable.Section{{Widgets: []deliverable.Widget{
		{Kind: deliverable.Light, Label: "Before", Tone: "positive", Word: "Fine"},
		{Kind: deliverable.Text, Markdown: b.String()},
		{Kind: deliverable.Light, Label: "After", Tone: "negative", Word: "Late"},
	}}}}
	if err := deliverable.Validate(s); err != nil {
		t.Fatal(err)
	}
	out, rep := render(t, s, Options{ModTime: modTime})
	if rep.Pages < 3 {
		t.Errorf("a long note took %d pages", rep.Pages)
	}
	text := strings.Join(pages(t, out), " ")
	if miss, ok := inOrder(text, []string{"Before", "Fine", "Paragraph 1 ", "Paragraph 60 ", "Paragraph 120 ", "After", "Late"}); !ok {
		t.Errorf("%q is missing or out of order", miss)
	}
}

func TestCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, _, err := Render(ctx, kitchenSink(t), Options{}); !errors.Is(err, context.Canceled) || out != nil {
		t.Errorf("a cancelled render = %d bytes, %v", len(out), err)
	}
}
