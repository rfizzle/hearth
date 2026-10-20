package template

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// appCopies is where the app keeps its copies of the shipped templates, the
// fixtures its native renderer is drawn and tested against.
const appCopies = "../../../App/Packages/HearthKit/Sources/HearthKit/Resources/Deliverables"

// TestAppCopiesEqualTheShipped fails when the app's copies of the shipped
// templates drift from the set the core embeds: the same file names, and each
// file the same bytes.
func TestAppCopiesEqualTheShipped(t *testing.T) {
	embedded, err := shippedFS.ReadDir("shipped")
	if err != nil {
		t.Fatal(err)
	}
	copies, err := os.ReadDir(appCopies)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(copies), names(embedded); !slices.Equal(got, want) {
		t.Fatalf("the app's copies are %q; the shipped set is %q", got, want)
	}
	for _, e := range embedded {
		want, err := shippedFS.ReadFile("shipped/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(appCopies, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the shipped template; copy it again from shipped/", e.Name())
		}
	}
}

func names(es []os.DirEntry) []string {
	ns := make([]string, 0, len(es))
	for _, e := range es {
		ns = append(ns, e.Name())
	}
	slices.Sort(ns)
	return ns
}
