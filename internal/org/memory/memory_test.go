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

func TestOneLine(t *testing.T) {
	if got := OneLine("a\nb\r\tc d é"); got != "a b  c d é" {
		t.Fatalf("OneLine = %q", got)
	}
}
