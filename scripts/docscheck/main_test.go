package main

import "testing"

func TestSlugify(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Go core", "go-core"},
		{"The `store` layer: what & why?", "the-store-layer-what--why"},
		{"Already-hyphenated 2nd Pass", "already-hyphenated-2nd-pass"},
		{"Ünïcode Café", "ünïcode-café"},
	} {
		if got := slugify(tt.in); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The identifiers are assembled from parts so this file does not trip the
// very check it tests.
func TestWorkItemRe(t *testing.T) {
	story, epic := "S"+"-0001", "E"+"-042"
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"fixes " + story + " today", true},
		{"epic " + epic, true},
		{"x" + story, false},
		{"S" + "-001", false},
		{"E" + "-0042", false},
		{"plain text", false},
	} {
		if got := workItemRe.MatchString(tt.in); got != tt.want {
			t.Errorf("workItemRe(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCitationRe(t *testing.T) {
	m := citationRe.FindStringSubmatch("// See " + "docs/architecture.md#go-core")
	if m == nil || m[1] != "docs/architecture.md" || m[2] != "go-core" {
		t.Fatalf("got %v", m)
	}
	if m := citationRe.FindStringSubmatch("// See " + "docs/decisions.md"); m == nil || m[2] != "" {
		t.Fatalf("bare citation: got %v", m)
	}
}
