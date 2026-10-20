package chart

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

// tokensFile is the design tokens the app and tokens-check read.
const tokensFile = "../../../App/Packages/HearthDesign/Sources/HearthDesign/Resources/tokens.json"

// TestPaletteMatchesTokens fails when palette.go drifts from tokens.json;
// make tokens-gen rewrites it.
func TestPaletteMatchesTokens(t *testing.T) {
	data, err := os.ReadFile(tokensFile)
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	var tokens struct {
		Colors map[string]struct{ Light, Dark string } `json:"colors"`
	}
	if err := json.Unmarshal(data, &tokens); err != nil {
		t.Fatalf("decode tokens: %v", err)
	}
	for i := range PaletteLight {
		name := "chart." + strconv.Itoa(i+1)
		want, ok := tokens.Colors[name]
		if !ok {
			t.Fatalf("tokens.json has no %s", name)
		}
		if PaletteLight[i] != want.Light || PaletteDark[i] != want.Dark {
			t.Errorf("%s: palette.go has %s/%s, tokens.json has %s/%s (run make tokens-gen)",
				name, PaletteLight[i], PaletteDark[i], want.Light, want.Dark)
		}
	}
}
