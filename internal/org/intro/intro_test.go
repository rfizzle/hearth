package intro

import (
	"strings"
	"testing"
)

func TestAssemble_QuotesTheFirstSentence(t *testing.T) {
	long := strings.Repeat("word ", 60) + "end."
	for _, tc := range []struct {
		name, role, desc, purpose string
	}{
		{"role and two sentences", "Bookkeeper",
			"Keeps my receipts and invoices in order and tells me what I owe each quarter. Prefers spreadsheets.",
			"I'm your Bookkeeper. The job, in your words: “Keeps my receipts and invoices in order and tells me what I owe each quarter.”"},
		{"no role", "", "Waters the plants!", "I'm on your team. The job, in your words: “Waters the plants!”"},
		{"no description", "Gardener", "  ", "I'm your Gardener."},
		{"no stop", "Gardener", "Waters the plants", "I'm your Gardener. The job, in your words: “Waters the plants”"},
		{"decimal is not a stop", "Gardener", "Spends 3.5 hours a week.", "I'm your Gardener. The job, in your words: “Spends 3.5 hours a week.”"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Assemble(tc.role, tc.desc)
			if got.Purpose != tc.purpose {
				t.Fatalf("purpose = %q, want %q", got.Purpose, tc.purpose)
			}
			if err := Validate(got); err != nil {
				t.Fatalf("an assembled introduction is invalid: %v", err)
			}
		})
	}
	t.Run("long sentence is cut at a word", func(t *testing.T) {
		got := Assemble("Gardener", long).Purpose
		quoted := got[strings.Index(got, "“")+len("“") : strings.Index(got, "”")]
		if !strings.HasSuffix(quoted, "word…") || len([]rune(quoted)) > 201 {
			t.Fatalf("quoted = %q (%d runes)", quoted, len([]rune(quoted)))
		}
	})
}

func TestValidate(t *testing.T) {
	good := Assemble("Writer", "Writes.")
	if err := Validate(good); err != nil {
		t.Fatalf("a good introduction: %v", err)
	}
	for name, edit := range map[string]func(*Introduction){
		"no purpose":     func(i *Introduction) { i.Purpose = " " },
		"two how lines":  func(i *Introduction) { i.How = i.How[:2] },
		"bad symbol":     func(i *Introduction) { i.How[0].Symbol = "Text Bubble" },
		"empty how text": func(i *Introduction) { i.How[1].Text = "" },
		"four prompts":   func(i *Introduction) { i.Try = append(i.Try, "More") },
		"empty prompt":   func(i *Introduction) { i.Try[0] = "" },
		"long prompt":    func(i *Introduction) { i.Try[2] = strings.Repeat("a", MaxPrompt+1) },
	} {
		t.Run(name, func(t *testing.T) {
			i := Introduction{Purpose: good.Purpose, How: append([]Line(nil), good.How...), Try: append([]string(nil), good.Try...)}
			edit(&i)
			if Validate(i) == nil {
				t.Fatal("Validate accepted it")
			}
		})
	}
}
