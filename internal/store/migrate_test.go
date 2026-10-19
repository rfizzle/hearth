package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// releasedMigrations is a digest of every migration a build has shipped, by
// position. A step inserted anywhere but the end moves every later one and
// fails here; so does editing one that has already run somewhere.
var releasedMigrations = []string{
	"93df9e5c9bf09887", // 1: org, bots, grants, conversations, runs, messages
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

// A second open of a current store runs no step: the version stays at the
// length of the list and the rows the first open wrote are still there.
func TestOpen_TwiceIsANoOpMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hearth.db")
	db := open(t, path)
	if err := db.PutOrg(t.Context(), orgFixture); err != nil {
		t.Fatal(err)
	}
	if got := versionRows(t, db); got != len(migrations) {
		t.Fatalf("after the first open: %d version rows, want %d", got, len(migrations))
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db = open(t, path)
	if got := versionRows(t, db); got != len(migrations) {
		t.Fatalf("after the second open: %d version rows, want %d", got, len(migrations))
	}
	if got, err := db.Org(t.Context()); err != nil || got != orgFixture {
		t.Fatalf("org after reopening = %+v, %v", got, err)
	}
}

func versionRows(t *testing.T, db *DB) int {
	t.Helper()
	var n, max int
	if err := db.sql.QueryRow(`SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_version`).Scan(&n, &max); err != nil {
		t.Fatal(err)
	}
	if n != max {
		t.Fatalf("schema_version has %d rows but records version %d", n, max)
	}
	return n
}
