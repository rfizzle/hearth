package fonts

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// TestFacesMatchSources holds the sums SOURCES.md records, so a font
// replaced without updating it fails here.
func TestFacesMatchSources(t *testing.T) {
	want := map[string]string{
		"SourceSans3-Regular.ttf":   "4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f",
		"SourceSans3-Bold.ttf":      "9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a",
		"SourceSans3-It.ttf":        "192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4",
		"SourceCodePro-Regular.ttf": "74bd80d3e42a08517cd7e1108ba3d86f2da29ac0f3065be95e0357956ab9db37",
	}
	got := Faces()
	if len(got) != len(want) {
		t.Fatalf("Faces has %d faces, want %d", len(got), len(want))
	}
	for _, f := range got {
		sum := sha256.Sum256(f.Data)
		if hex.EncodeToString(sum[:]) != want[f.File] {
			t.Errorf("%s: sha256 %x, want %s", f.File, sum, want[f.File])
		}
	}
	got[0].Data[0] ^= 0xff
	if again := Faces(); again[0].Data[0] == got[0].Data[0] {
		t.Error("a change to one Faces result reached the next")
	}
}

func TestLicense(t *testing.T) {
	l := License()
	for _, want := range []string{"SIL OPEN FONT LICENSE Version 1.1", "Copyright 2010-2022 Adobe", "© 2023 Adobe"} {
		if !strings.Contains(l, want) {
			t.Errorf("the licence is missing %q", want)
		}
	}
}
