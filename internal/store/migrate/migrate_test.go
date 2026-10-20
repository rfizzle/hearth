package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// releasedMigrations is a digest of every migration a build has shipped, by
// position. A step inserted anywhere but the end moves every later one and
// fails here; so does editing one that has already run somewhere.
var releasedMigrations = []string{
	"93df9e5c9bf09887", // 1: org, bots, grants, conversations, runs, messages
	"f56ea3ed3d7ca9cc", // 2: inbox items, allowances, the message widget columns
	"2b92f32667289f55", // 3: notes, handbook pages, usage, budgets, tiers, summaries, bot override, attachments
	"6e62aca3a00a7458", // 4: the bot scope
}

func migrationDigest(m string) string {
	sum := sha256.Sum256([]byte(m))
	return hex.EncodeToString(sum[:8])
}

// The list is held append-only. The recorded version is a count, so a step
// inserted below it is recorded as done on every store already past that
// position and never runs there.
func TestMigrations_AreAppendOnly(t *testing.T) {
	const rule = "migrations are appended and never inserted, removed or edited: " +
		"schema_version is a count, so a step placed below it is recorded as done on every store already past it without running. " +
		"Put the change at the end of the list as a new step"
	if len(migrations) < len(releasedMigrations) {
		t.Fatalf("%d migrations, but %d were released — %s", len(migrations), len(releasedMigrations), rule)
	}
	for i, want := range releasedMigrations {
		if got := migrationDigest(migrations[i]); got != want {
			t.Fatalf("migration %d is not the one released (digest %s, want %s) — %s", i+1, got, want, rule)
		}
	}
	if len(migrations) > len(releasedMigrations) {
		var add []string
		for _, m := range migrations[len(releasedMigrations):] {
			add = append(add, fmt.Sprintf("%q,", migrationDigest(m)))
		}
		t.Fatalf("%d new migration(s) appended; add their digests to releasedMigrations: %s",
			len(migrations)-len(releasedMigrations), strings.Join(add, " "))
	}
}
