package picture

import (
	"context"
	"reflect"
	"testing"
)

func TestPageList(t *testing.T) {
	for _, tc := range []struct {
		pages []int
		want  string
	}{
		{[]int{3}, "3"}, {[]int{3, 7}, "3 and 7"}, {[]int{1, 2, 3}, "1-3"},
		{[]int{1, 2, 3, 5, 9}, "1-3, 5 and 9"}, {[]int{4, 5}, "4-5"},
	} {
		if got := PageList(tc.pages); got != tc.want {
			t.Errorf("PageList(%v) = %q, want %q", tc.pages, got, tc.want)
		}
	}
}

type answers func(pages []int) ([]Recognised, error)

func (f answers) Recognise(_ context.Context, _ string, pages []int) ([]Recognised, error) {
	return f(pages)
}

func TestOfferKeepsOnlyWhatWasAsked(t *testing.T) {
	rec := answers(func(pages []int) ([]Recognised, error) {
		return []Recognised{{Page: 2, Text: "a"}, {Page: 2, Text: "again"}, {Page: 9, Text: "not asked"}}, nil
	})
	got := Offer(context.Background(), rec, "/a.pdf", []int{1, 2})
	if !reflect.DeepEqual(got.Got, map[int]Recognised{2: {Page: 2, Text: "a"}}) || !reflect.DeepEqual(got.Missed, []int{1}) || got.Absent != nil {
		t.Errorf("Offer = %+v", got)
	}
}

func TestOfferStopsWhenTheAppIsGoneOrTimeIsUp(t *testing.T) {
	calls := 0
	gone := answers(func([]int) ([]Recognised, error) { calls++; return nil, ErrNoApp })
	pages := make([]int, 25)
	for i := range pages {
		pages[i] = i + 1
	}
	if got := Offer(context.Background(), gone, "/a.pdf", pages); calls != 1 || len(got.Absent) != 25 {
		t.Errorf("no app: calls %d, absent %d; want 1 and 25", calls, len(got.Absent))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if got := Offer(ctx, gone, "/a.pdf", pages); calls != 0 || len(got.Missed) != 25 {
		t.Errorf("time up: calls %d, missed %d; want 0 and 25", calls, len(got.Missed))
	}
}

func TestImageNotice(t *testing.T) {
	for _, tc := range []struct {
		app, answered, text bool
		want                string
	}{
		{false, false, false, "[a.png is a picture. Hearth isn't open, so its text could not be read.]"},
		{true, false, false, "[a.png is a picture; its text could not be recognised.]"},
		{true, true, false, "[a.png is a picture; Hearth found no text in it.]"},
		{true, true, true, "[a.png is a picture; its text was recognised by Hearth and may have mistakes.]"},
	} {
		if got := Image("a.png", tc.app, tc.answered, tc.text); got != tc.want {
			t.Errorf("Image(%v, %v, %v) = %q", tc.app, tc.answered, tc.text, got)
		}
	}
}
