package store

import (
	"reflect"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/intro"
)

// A bot keeps the introduction it was hired with, even after its role is
// edited, and one hired before introductions existed reads as an assembled one.
func TestBot_KeepsTheIntroductionItWasHiredWith(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()

	bot, grants, dm := org.NewBot("Dana", "Writer", "Writes.", org.TierBalanced, orgFixture.WorkspacePath)
	bot.Introduction = intro.Introduction{
		Purpose: "I write.",
		How:     []intro.Line{{Symbol: "folder", Text: "a"}, {Symbol: "pencil.line", Text: "b"}, {Symbol: "doc.text", Text: "c"}},
		Try:     []string{"one", "two", "three"},
	}
	stored, err := db.CreateBot(ctx, bot, grants, dm)
	if err != nil {
		t.Fatal(err)
	}
	stored.Role = "Editor"
	if _, err := db.UpdateBot(ctx, stored); err != nil {
		t.Fatal(err)
	}
	got, err := db.Bot(ctx, stored.ID)
	if err != nil || !reflect.DeepEqual(got.Introduction, bot.Introduction) {
		t.Fatalf("introduction = %+v, %v; want the one hired with", got.Introduction, err)
	}

	if _, err := db.sql.ExecContext(ctx, `UPDATE bots SET introduction = '' WHERE id = ?`, stored.ID); err != nil {
		t.Fatal(err)
	}
	got, err = db.Bot(ctx, stored.ID)
	if err != nil || got.Introduction.Purpose != intro.Assemble("Editor", "Writes.").Purpose {
		t.Fatalf("legacy introduction = %+v, %v; want the assembled one", got.Introduction, err)
	}
}
