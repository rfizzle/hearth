package skill

import "testing"

func TestTemplates_LoadsTheThreeRoles(t *testing.T) {
	got := Templates()
	want := []string{"planner", "research-analyst", "writer"}
	if len(got) != len(want) {
		t.Fatalf("Templates = %d, want %d", len(got), len(want))
	}
	for i, tpl := range got {
		if tpl.ID != want[i] {
			t.Errorf("template %d id = %q, want %q", i, tpl.ID, want[i])
		}
		if tpl.Role == "" || tpl.Summary == "" || tpl.JobDescription == "" || tpl.SuggestedTier == 0 {
			t.Errorf("template %s has an empty field: %+v", tpl.ID, tpl)
		}
	}
}
