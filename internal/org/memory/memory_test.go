package memory

import "testing"

func TestSelect_EveryoneThenTheBot(t *testing.T) {
	pages := []Page{
		{ID: "1", Scope: Scope{BotID: "dana"}},
		{ID: "2", Scope: Scope{Everyone: true}},
		{ID: "3", Scope: Scope{BotID: "sam"}},
		{ID: "4", Scope: Scope{Everyone: true}},
		{ID: "5", Scope: Scope{BotID: "dana"}},
	}
	var got string
	for _, p := range Select(pages, "dana") {
		got += p.ID
	}
	if got != "24"+"15" {
		t.Fatalf("Select = %q, want 2415", got)
	}
	if n := len(Select(pages, "")); n != 2 {
		t.Fatalf("a reader with no id sees %d pages, want only the 2 for everyone", n)
	}
}

func TestMeter(t *testing.T) {
	used, limit := Meter([]Page{{Markdown: "abc"}, {Markdown: "héllo"}})
	if used != 9 || limit != 24*1024 {
		t.Fatalf("Meter = %d, %d", used, limit)
	}
}

func TestPlain(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"plain text", "Ship the newsletter, é.", "Ship the newsletter, é."},
		{"a right-to-left override", "a\u202eb", "ab"},
		{"the isolates", "a\u2066b\u2067c\u2068d\u2069e", "abcde"},
		{"a zero-width space", "a\u200bb", "ab"},
		{"a byte order mark", "\ufeffab", "ab"},
		{"a tag character", "a\U000E0041b", "ab"},
		{"a NUL", "a\x00b", "ab"},
		{"the next-line control", "a\u0085b", "ab"},
		{"a carriage return", "a\rb", "ab"},
		{"an escape", "a\x1b[31mb", "a[31mb"},
		{"a ZWJ emoji sequence is kept", "👩\u200d💻", "👩\u200d💻"},
		{"a newline and a tab are kept", "a\n\tb", "a\n\tb"},
		{"a line separator is not format or control", "a\u2028b", "a\u2028b"},
		{"a selector smuggled after a letter is dropped when doubled", "a\ufe0f\ufe0fb", "a\ufe0fb"},
		{"a selector at the start", "\ufe00a", "a"},
		{"each variation selector of the first block", "a\ufe00b\ufe01c\ufe0dd", "abcd"},
		{"a selector of the supplement", "a\U000E0100b\U000E01EFc", "abc"},
		{"one emoji presentation selector is kept", "\u2764\ufe0f", "\u2764\ufe0f"},
		{"one text presentation selector is kept", "\u2764\ufe0e", "\u2764\ufe0e"},
		{"a keycap sequence is kept", "1\ufe0f\u20e3", "1\ufe0f\u20e3"},
		{"a presentation selector after a space is dropped", " \ufe0f", " "},
		{"a presentation selector after a line break is dropped", "\n\ufe0f", "\n"},
		{"a combining grapheme joiner", "a\u034fb", "ab"},
		{"the hangul fillers", "a\u115f\u1160b", "ab"},
		{"a khmer inherent vowel", "a\u17b4\u17b5b", "ab"},
		{"the mongolian selectors", "a\u180b\u180c\u180d\u180fb", "ab"},
		{"the hangul filler letters", "a\u3164\uffa0b", "ab"},
		{"the unassigned tag block", "a\U000E0080b\U000E0FFFc", "abc"},
		{"a selector after a dropped rune is dropped", "a\u200b\ufe0f", "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Plain(tc.in); got != tc.want {
				t.Errorf("Plain(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOneLine(t *testing.T) {
	if got := OneLine("a\nb\r\tc d é"); got != "a b  c d é" {
		t.Fatalf("OneLine = %q", got)
	}
}
