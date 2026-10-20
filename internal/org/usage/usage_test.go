package usage

import (
	"testing"
	"time"
)

func TestMonth(t *testing.T) {
	if got := Month(time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)); got != "2026-10" {
		t.Fatalf("Month = %q", got)
	}
}

func TestLevel(t *testing.T) {
	b := Budget{Subject: Org, Cents: 2000} // $20 = 20,000,000 micros
	for _, tc := range []struct {
		name       string
		micros     int64
		budget     Budget
		frac       float64
		soft, hard bool
	}{
		{"under", 10_000_000, b, 0.5, false, false},
		{"at soft", 16_000_000, b, 0.8, true, false},
		{"over soft", 19_000_000, b, 0.95, true, false},
		{"at hard", 20_000_000, b, 1, true, true},
		{"no budget", 99_000_000, Budget{Subject: Org}, 0, false, false},
	} {
		frac, soft, hard := Level(Totals{CostMicros: tc.micros}, tc.budget)
		if frac != tc.frac || soft != tc.soft || hard != tc.hard {
			t.Errorf("%s: Level = %v, %v, %v, want %v, %v, %v", tc.name, frac, soft, hard, tc.frac, tc.soft, tc.hard)
		}
	}
}
