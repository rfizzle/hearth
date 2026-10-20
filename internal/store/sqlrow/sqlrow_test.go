package sqlrow

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestMillis_ZeroIsNullAndRoundTrips(t *testing.T) {
	if got := Millis(time.Time{}); got.Valid {
		t.Errorf("Millis(zero) = %+v, want NULL", got)
	}
	if got := FromMillis(sql.NullInt64{}); !got.IsZero() {
		t.Errorf("FromMillis(NULL) = %v, want zero", got)
	}
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	if got := FromMillis(Millis(at)); !got.Equal(at) {
		t.Errorf("round trip = %v, want %v", got, at)
	}
}

func TestNullString_EmptyIsNull(t *testing.T) {
	if NullString("").Valid || !NullString("x").Valid {
		t.Error("NullString: want empty as NULL and anything else valid")
	}
}

func TestNotFound(t *testing.T) {
	if !errors.Is(NotFound(sql.ErrNoRows), ErrNotFound) || !errors.Is(OrNotFound(nil), ErrNotFound) {
		t.Error("no rows and no error should both read as ErrNotFound")
	}
	other := errors.New("disk")
	if !errors.Is(NotFound(other), other) || !errors.Is(OrNotFound(other), other) {
		t.Error("another error should pass through")
	}
	if a, b := NewID(), NewID(); len(a) != 32 || a == b {
		t.Errorf("NewID = %q, %q, want two different 32-character ids", a, b)
	}
}
