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
