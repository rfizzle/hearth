package store

import (
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func TestBackfillAskOwner_GivesTheRightOnlyToBotsWithoutIt(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	old := hire(t, db, "Dana")
	kept := hire(t, db, "Sam")
	if _, err := db.sql.ExecContext(ctx, `DELETE FROM grants WHERE bot_id = ? AND capability = ?`, old.ID, org.AskOwner); err != nil {
		t.Fatal(err)
	}
	// Sam's was switched off; that is a choice the backfill respects.
	if _, err := db.sql.ExecContext(ctx, `UPDATE grants SET mode = ? WHERE bot_id = ? AND capability = ?`, org.ModeNotGranted, kept.ID, org.AskOwner); err != nil {
		t.Fatal(err)
	}

	n, err := db.BackfillAskOwner(ctx)
	if err != nil || n != 1 {
		t.Fatalf("BackfillAskOwner = %d, %v, want 1", n, err)
	}
	if grants, _ := db.GrantsFor(ctx, old.ID); org.Allowed(grants, org.AskOwner) != org.ModeAllowed {
		t.Errorf("the old bot's grants = %v, want the right to ask, allowed", grants)
	}
	if grants, _ := db.GrantsFor(ctx, kept.ID); org.Allowed(grants, org.AskOwner) != org.ModeNotGranted {
		t.Errorf("the other bot's grants = %v, want its switched-off grant untouched", grants)
	}
	if n, err := db.BackfillAskOwner(ctx); err != nil || n != 0 {
		t.Errorf("BackfillAskOwner again = %d, %v, want 0", n, err)
	}
}
