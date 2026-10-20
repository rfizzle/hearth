package deliverable

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// table is a table of cols columns and rows one-cell rows, which the parser
// pads to the full width.
func table(cols, rows int) string {
	return strings.Repeat("|a", cols) + "|\n" + strings.Repeat("|-", cols) + "|\n" + strings.Repeat("|\n", rows)
}

func TestCheckMarkdownRefuses(t *testing.T) {
	for name, src := range map[string]string{
		"brackets":  strings.Repeat("[", maxBrackets+1),
		"emphasis":  strings.Repeat("a*_", maxDelimiters/2+1),
		"columns":   table(maxColumns+1, 1),
		"cells":     table(maxColumns, 1600),
		"quotes":    strings.Repeat(">", maxLeading+1) + " deep",
		"lists":     strings.Repeat("- ", maxLeading) + "deep",
		"items":     "- a\n" + strings.Repeat("  ", 200) + "- b\n",
		"in a list": "- " + table(maxColumns+1, 1),
	} {
		start := time.Now()
		if err := CheckMarkdown([]byte(src)); !errors.Is(err, ErrTooComplex) {
			t.Errorf("%s: %v, want ErrTooComplex", name, err)
		}
		if d := time.Since(start); d > time.Second {
			t.Errorf("%s: the check took %v", name, d)
		}
	}
}

// TestCheckMarkdownAccepts parses inputs just under each bound and holds
// the parse to a time and an allocation measured with room to spare: about
// 1.5 s and 15 MB for the emphasis, 10 ms and 16 MB for the table.
func TestCheckMarkdownAccepts(t *testing.T) {
	for name, src := range map[string]string{
		"emphasis": strings.Repeat("a*_", maxDelimiters/2),
		"cells":    table(maxColumns, 1500),
		"brackets": strings.Repeat("[a](", maxBrackets),
		"nesting":  strings.Repeat("> ", 120) + "deep",
		"a rule":   strings.Repeat("-", 5000),
		"a number": strings.Repeat("7", 5000),
		"prose":    strings.Repeat("A plain *line* with _a_ few ~~marks~~ and [a link](https://example.com).\n", 2000),
	} {
		if err := CheckMarkdown([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		ParseMarkdown([]byte(src))
		took := time.Since(start)
		runtime.ReadMemStats(&after)
		if took > 20*time.Second {
			t.Errorf("%s: the parse took %v", name, took)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 128<<20 {
			t.Errorf("%s: the parse allocated %d MB", name, alloc>>20)
		}
	}
}

func TestValidateRefusesComplexMarkdown(t *testing.T) {
	err := markdown("text", strings.Repeat("|a", 70)+"|\n"+strings.Repeat("|-", 70)+"|\n", 20000)
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "more than 64 columns") {
		t.Errorf("markdown = %v", err)
	}
}
