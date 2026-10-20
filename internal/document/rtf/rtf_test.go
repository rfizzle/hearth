package rtf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strip(t *testing.T, in string, limit int) (string, bool, error) {
	t.Helper()
	return Strip(context.Background(), strings.NewReader(in), limit)
}

func TestStrip(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain", `{\rtf1 Hello world}`, "Hello world"},
		{"paragraphs and tabs", `{\rtf1 a\par b\line c\tab d}`, "a\nb\nc\td"},
		{"hex escape is windows-1252", `{\rtf1 caf\'e9 \'80 5 \'93hi\'94}`, "café € 5 “hi”"},
		{"bad hex is dropped", `{\rtf1 a\'zzb}`, "ab"},
		{"unicode with a fallback", `{\rtf1 \u8364?12 \u233 e}`, "€12 é"},
		{"negative unicode", `{\rtf1 \u-10179?\u-8704?}`, "😀"},
		{"uc0 has no fallback", `{\rtf1\uc0 \u8364 12}`, "€12"},
		{"uc2 skips two", `{\rtf1\uc2 \u8364 ab12}`, "€12"},
		{"hex fallback is skipped", `{\rtf1 \u233\'e9x}`, "éx"},
		{"escaped specials", `{\rtf1 \\ \{ \} \~ \_}`, `\ { }   -`},
		{"font table", `{\rtf1{\fonttbl{\f0 Arial;}}Text}`, "Text"},
		{"colour table", `{\rtf1{\colortbl;\red0\green0\blue0;}Text}`, "Text"},
		{"stylesheet", `{\rtf1{\stylesheet{\s1 Heading;}}Text}`, "Text"},
		{"info", `{\rtf1{\info{\title Secret}}Text}`, "Text"},
		{"starred destination", `{\rtf1 A{\*\generator Word;}B}`, "AB"},
		{"nested starred group", `{\rtf1 A{\*\foo {\bar nested text}}B}`, "AB"},
		{"picture", `{\rtf1 A{\pict\pngblip 89504e47}B}`, "AB"},
		{"field keeps its result only", `{\rtf1{\field{\*\fldinst HYPERLINK "x"}{\fldrslt link text}}}`, "link text"},
		{"group formatting keeps text", `{\rtf1 a {\b bold} c}`, "a bold c"},
		{"bin data is skipped", "{\\rtf1 a\\bin3 }}}b}", "ab"},
		{"line breaks in the file are not text", "{\\rtf1 a\r\nb}", "ab"},
		{"many blank lines fold", `{\rtf1 a\par\par\par\par b}`, "a\n\nb"},
		{"cell and row", `{\rtf1 a\cell b\cell\row c}`, "a\tb\t\nc"},
		{"a long word is junk", `{\rtf1 a\` + strings.Repeat("x", 500) + ` b}`, "ab"},
		{"digits are capped", `{\rtf1 a\` + `u99999999999999999999 b}`, "a\xef\xbf\xbd"},
		{"unbalanced closing braces", `{\rtf1 a}}}b`, "ab"},
		{"truncated escape", `{\rtf1 a\'e`, "a"},
	}
	for _, tc := range tests {
		got, cut, err := strip(t, tc.in, 1<<20)
		if err != nil || cut || got != tc.want {
			t.Errorf("%s: %q, cut %v, %v; want %q", tc.name, got, cut, err, tc.want)
		}
	}
}

func TestStripNotRTF(t *testing.T) {
	for _, in := range []string{"", "plain text", `{\rt`, `\rtf1 x`, "%PDF-1.4"} {
		if _, _, err := strip(t, in, 100); !errors.Is(err, ErrNotRTF) {
			t.Errorf("%q: err %v, want ErrNotRTF", in, err)
		}
	}
}

func TestStripDepthCap(t *testing.T) {
	ok := `{\rtf1` + strings.Repeat("{", MaxDepth-2) + "x" + strings.Repeat("}", MaxDepth-2) + "}"
	if got, _, err := strip(t, ok, 100); err != nil || got != "x" {
		t.Errorf("depth %d: %q, %v; want x", MaxDepth-1, got, err)
	}
	deep := `{\rtf1` + strings.Repeat("{", MaxDepth) + "x"
	if _, _, err := strip(t, deep, 100); !errors.Is(err, ErrTooDeep) {
		t.Errorf("depth %d: err %v, want ErrTooDeep", MaxDepth+1, err)
	}
	// Dropped groups count toward the depth too.
	if _, _, err := strip(t, `{\rtf1{\fonttbl`+strings.Repeat("{", MaxDepth)+"}", 100); !errors.Is(err, ErrTooDeep) {
		t.Errorf("a deep dropped group: err %v, want ErrTooDeep", err)
	}
}

func TestStripSizeCap(t *testing.T) {
	in := `{\rtf1 ` + strings.Repeat("word ", 1000) + `}`
	got, cut, err := strip(t, in, 100)
	if err != nil || !cut || len(got) > 100 || !strings.HasPrefix(got, "word word") {
		t.Errorf("cap 100: %d bytes, cut %v, %v", len(got), cut, err)
	}
	// A cap reached inside a dropped group does not count its text.
	got, cut, err = strip(t, `{\rtf1 ab{\fonttbl`+strings.Repeat("x", 500)+`}cd}`, 4)
	if err != nil || cut || got != "abcd" {
		t.Errorf("dropped text: %q, cut %v, %v", got, cut, err)
	}
}

func TestStripBinaryRunIsNotAllocated(t *testing.T) {
	if got, _, err := strip(t, `{\rtf1 a\bin2000000000 b`, 100); err != nil || got != "a" {
		t.Errorf("a \\bin past the end: %q, %v", got, err)
	}
}

func TestStripEndsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in := `{\rtf1 ` + strings.Repeat("x", 3*checkEvery) + `}`
	if _, _, err := Strip(ctx, strings.NewReader(in), 1<<30); !errors.Is(err, context.Canceled) {
		t.Errorf("err %v, want the context's", err)
	}
}

func TestStripFixture(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "testdata", "letter.rtf"))
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := Strip(context.Background(), strings.NewReader(string(b)), 1<<20)
	want := "Café menu\nPrice: €12\tnet\nBold done."
	if err != nil || got != want {
		t.Errorf("letter.rtf: %q, %v; want %q", got, err, want)
	}
}
