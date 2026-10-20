package project

import (
	"errors"
	"testing"
)

func TestCanBecome(t *testing.T) {
	cases := []struct {
		from, to State
		ok       bool
	}{
		{Active, Done, true},
		{Proposed, Done, false},
		{Done, Done, false},
		{Archived, Done, false},
		{Proposed, Archived, true},
		{Active, Archived, true},
		{Done, Archived, true},
		{Archived, Archived, false},
		{Active, Active, false},
	}
	for _, c := range cases {
		err := c.from.CanBecome(c.to)
		if c.ok != (err == nil) || (err != nil && !errors.Is(err, ErrState)) {
			t.Errorf("%v -> %v = %v, want ok %v", c.from, c.to, err, c.ok)
		}
	}
}

func TestEdit_KeepsTheFolderAndRefusesWhatValidateRefuses(t *testing.T) {
	p := Project{ID: "p", Name: "Old", FolderPath: "/Hearth/Projects/Old", State: Active}
	got, err := Edit(p, " New ", goodBrief, "2026-12-31")
	if err != nil || got.Name != "New" || got.FolderPath != p.FolderPath || got.ID != "p" || got.TargetDate != "2026-12-31" {
		t.Fatalf("Edit = %+v, %v", got, err)
	}
	if _, err := Edit(p, "", goodBrief, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Edit with no name = %v, want ErrInvalid", err)
	}
}

func TestActivate_OnlyAProposal(t *testing.T) {
	for _, s := range []State{Active, Done, Archived} {
		if _, _, err := Activate(Project{ID: "p", State: s}, "P", goodBrief, "", "/Hearth"); !errors.Is(err, ErrState) {
			t.Errorf("Activate from %v = %v, want ErrState", s, err)
		}
	}
}
