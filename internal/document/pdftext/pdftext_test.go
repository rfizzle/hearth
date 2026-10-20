package pdftext

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func open(t *testing.T, name string) (*Reader, error) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	return Open(f, info.Size())
}

func TestOpen(t *testing.T) {
	for _, tc := range []struct {
		file string
		want error
	}{
		{"text3.pdf", nil},
		{"empty-password.pdf", nil},
		{"scan.pdf", nil},
		{"locked.pdf", ErrPassword},
		{"truncated.pdf", ErrDamaged},
	} {
		t.Run(tc.file, func(t *testing.T) {
			if _, err := open(t, tc.file); !errors.Is(err, tc.want) {
				t.Errorf("Open = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPageText(t *testing.T) {
	for file, want := range map[string]string{
		"text3.pdf":          "Second page: the quarterly figures",
		"empty-password.pdf": "Second page: the quarterly figures",
	} {
		r, err := open(t, file)
		if err != nil {
			t.Fatal(err)
		}
		if n := r.NumPage(); n != 3 {
			t.Errorf("%s: NumPage = %d, want 3", file, n)
		}
		got, err := r.Page(context.Background(), 2)
		if err != nil || !strings.Contains(got, want) {
			t.Errorf("%s: page 2 = %q, %v; want it to contain %q", file, got, err, want)
		}
		if _, err := r.Page(context.Background(), 9); !errors.Is(err, ErrPage) {
			t.Errorf("%s: page 9 err = %v, want ErrPage", file, err)
		}
	}
	r, err := open(t, "scan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.Page(context.Background(), 1); err != nil || strings.TrimSpace(got) != "" {
		t.Errorf("scan page = %q, %v; want no text", got, err)
	}
}

func TestPageContainsAPanic(t *testing.T) {
	r := &Reader{parse: func(int) (string, error) { panic("crafted") }}
	if _, err := r.Page(context.Background(), 1); !errors.Is(err, ErrPage) {
		t.Errorf("err = %v, want ErrPage", err)
	}
}

func TestPageGivesUpWhenTheContextEnds(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	r := &Reader{parse: func(int) (string, error) { <-block; return "", nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.Page(ctx, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the deadline", err)
	}
	if _, err := r.Page(ctx, 2); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("after the deadline: err = %v, want the deadline without a parse", err)
	}
}
