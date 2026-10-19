// Command tokenscheck fails when a colour pairing in the design tokens falls
// below WCAG 2.2 AA contrast for body text (4.5:1) in light or dark
// appearance, or names a colour the tokens do not define. It prints every
// ratio it checks.
//
// Usage: tokenscheck <tokens.json>
//
// The file holds "colors", a map from role to {"light", "dark"} sRGB hex
// values, and "pairings", a list of {"foreground", "background"} role names.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// minContrast is WCAG 2.2 AA's minimum for normal-size text.
const minContrast = 4.5

type colorPair struct {
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

type pairing struct {
	Foreground string `json:"foreground"`
	Background string `json:"background"`
}

type tokens struct {
	Colors   map[string]colorPair `json:"colors"`
	Pairings []pairing            `json:"pairings"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tokenscheck <tokens.json>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := check(data, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// check reports every pairing's contrast in both appearances to w and returns
// an error naming each failure.
func check(data []byte, w io.Writer) error {
	var t tokens
	if err := json.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("tokens: %w", err)
	}
	if len(t.Pairings) == 0 {
		return fmt.Errorf("tokens: no pairings to check")
	}
	var failures []string
	for _, p := range t.Pairings {
		fg, okFG := t.Colors[p.Foreground]
		bg, okBG := t.Colors[p.Background]
		if !okFG || !okBG {
			failures = append(failures, fmt.Sprintf("%s on %s: unknown colour role", p.Foreground, p.Background))
			continue
		}
		for _, a := range []struct{ name, fg, bg string }{{"light", fg.Light, bg.Light}, {"dark", fg.Dark, bg.Dark}} {
			ratio, err := contrast(a.fg, a.bg)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s on %s (%s): %v", p.Foreground, p.Background, a.name, err))
				continue
			}
			fmt.Fprintf(w, "%-20s on %-20s %-5s %5.2f:1\n", p.Foreground, p.Background, a.name, ratio)
			if ratio < minContrast {
				failures = append(failures, fmt.Sprintf("%s on %s (%s): %.2f:1 is below %.1f:1", p.Foreground, p.Background, a.name, ratio, minContrast))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("contrast check failed:\n  %s", strings.Join(failures, "\n  "))
	}
	return nil
}

// contrast is the WCAG 2.2 contrast ratio between two sRGB hex colours.
func contrast(a, b string) (float64, error) {
	la, err := luminance(a)
	if err != nil {
		return 0, err
	}
	lb, err := luminance(b)
	if err != nil {
		return 0, err
	}
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05), nil
}

// luminance is the WCAG 2.2 relative luminance of an sRGB colour written
// #RRGGBB.
func luminance(hex string) (float64, error) {
	s, ok := strings.CutPrefix(hex, "#")
	if !ok || len(s) != 6 {
		return 0, fmt.Errorf("colour %q is not #RRGGBB", hex)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("colour %q is not #RRGGBB", hex)
	}
	channel := func(c uint64) float64 {
		x := float64(c) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b := channel(v>>16&0xFF), channel(v>>8&0xFF), channel(v&0xFF)
	return 0.2126*r + 0.7152*g + 0.0722*b, nil
}
