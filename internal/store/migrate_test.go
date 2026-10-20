package store

import (
	"path/filepath"
	"testing"

	"github.com/rfizzle/hearth/internal/store/migrate"
)

// A second open of a current store runs no step: the version stays at the
// length of the list and the rows the first open wrote are still there.
func TestOpen_TwiceIsANoOpMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hearth.db")
	db := open(t, path)
	if err := db.PutOrg(t.Context(), orgFixture); err != nil {
		t.Fatal(err)
	}
	if got := versionRows(t, db); got != migrate.Count() {
		t.Fatalf("after the first open: %d version rows, want %d", got, migrate.Count())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db = open(t, path)
	if got := versionRows(t, db); got != migrate.Count() {
		t.Fatalf("after the second open: %d version rows, want %d", got, migrate.Count())
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
